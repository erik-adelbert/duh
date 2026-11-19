package mod

// MPTM applies volume-column slides before retriggering on the same tick.
func (w *Tracker) mptRetriggerAfterVolume(cmd ModCommand) bool {
	return w.Assets.ITCompatibility.mptMixProfile != nil && (cmd.EffectCmd == 0x1a || cmd.EffectCmd == 0x1b) &&
		cmd.HasVolume && cmd.Volume >= 65 && cmd.Volume <= 104
}

func applyITRetrigger(w *Tracker, track TrackID, voice VoiceID, cmd ModCommand) {
	mem := &w.TrackMemories[track]

	if cmd.EffectArg != 0 {
		mem.LastRetrig = cmd.EffectArg
	}

	arg := mem.LastRetrig
	interval := int(arg & 15)
	retrigger := false

	switch {
	case w.Assets.ITCompatibility.LegacyModPlug || w.Assets.ITCompatibility.mptMixProfile != nil && !w.Assets.ITCompatibility.mptITRetrigger:
		interval = max(interval, 1)
		retrigger = mem.RetrigCount > 0 && mem.RetrigCount%interval == 0
		mem.RetrigCount = (mem.RetrigCount + 1) & 255
	case w.Clock.Tick == 0 && cmd.Note != 0:
		mem.RetrigCount = interval
	default:
		if mem.RetrigCount > 0 {
			mem.RetrigCount--
		}

		if mem.RetrigCount == 0 {
			mem.RetrigCount = interval
			retrigger = true
		}
	}

	if !retrigger || voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	pb := &w.VoiceSamples[voice]

	if pb.oplChannel != 0 {
		w.opl.noteOff(pb.oplChannel - 1)
		w.VoiceLives[voice].KeyOff = false
	}

	if cmd.EffectCmd == 0x1a && arg == 0 {
		w.VoiceLives[voice].KeyOff = false
	}

	pb.Position, pb.Reverse = 0, false
	pb.mptLooped = false
	if arg>>4 == 0 || arg>>4 == 8 {
		return
	}

	amp := &w.VoiceAmps[voice]
	volume := int(amp.Volume * 256)
	switch mode := arg >> 4; mode {
	case 1, 2, 3, 4, 5:
		volume -= 4 << (mode - 1)
	case 6:
		volume = volume * 10 / 16
	case 7:
		volume /= 2
	case 9, 10, 11, 12, 13:
		volume += 4 << (mode - 9)
	case 14:
		volume = volume * 24 / 16
	case 15:
		volume *= 2
	}

	amp.Volume = float32(max(0, min(256, volume))) / 256

	if w.Assets.ITCompatibility.mptMixProfile != nil {
		amp.itSmoothRamp = false
	}
}
