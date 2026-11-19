package mod

func (w *Tracker) applyS3MTremor(track TrackID, voice VoiceID, cmd ModCommand) {
	active := voice != -1 && w.VoiceLives[voice].Active
	if active {
		w.VoiceAmps[voice].itTremorMuted = false
	}
	if cmd.EffectCmd != 0x22 {
		return
	}
	mem := &w.TrackMemories[track]
	if cmd.EffectArg != 0 {
		mem.itTremorParam = cmd.EffectArg
	}
	if !active {
		return
	}
	on := int(mem.itTremorParam>>4) + 1
	period := on + int(mem.itTremorParam&15) + 1
	count := int(mem.itTremorCount) % period
	w.VoiceAmps[voice].itTremorMuted = count >= on
	mem.itTremorCount = uint8(count + 1)
}
