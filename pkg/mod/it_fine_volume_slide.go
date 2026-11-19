package mod

func applyITFineVolumeSlide(w *Tracker, track TrackID, voice VoiceID, command uint8) {
	up := command < 75
	base := uint8(75)
	if up {
		base = 65
	}

	slide := command - base
	mem := &w.TrackMemories[track]

	if slide != 0 {
		mem.ITVolSlide = slide
	} else {
		slide = mem.ITVolSlide
	}

	if w.Assets.ITCompatibility.LegacyModPlug {
		if slide != 0 {
			mem.ITFineVolSlide = slide
		} else {
			slide = mem.ITFineVolSlide
		}
	}

	if voice == -1 || !w.VoiceLives[voice].Active || w.Assets.ITCompatibility.LegacyModPlug && w.Clock.Tick != 0 {
		return
	}

	delta := float32(slide) / 64
	if !up {
		delta = -delta
	}

	amp := &w.VoiceAmps[voice]
	amp.Volume = clamp32(amp.Volume+delta, 0, 1)
	amp.itSmoothRamp = slide != 0 || amp.itSmoothRamp
}
