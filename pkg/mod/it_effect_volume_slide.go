// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func applyITEffectVolumeSlide(w *Tracker, track TrackID, voice VoiceID, arg uint8) {
	applyITEffectVolumeSlideAtTick(w, track, voice, arg, w.itRepeatedRowTick())
}

func applyITEffectVolumeSlideAtTick(w *Tracker, track TrackID, voice VoiceID, arg uint8, tick int) {
	mem := &w.TrackMemories[track]
	if w.Assets.ITCompatibility.LegacyModPlug && arg&15 != 0 && arg&15 != 15 && arg>>4 != 0 && arg>>4 != 15 {
		arg &= 15
	}

	if arg != 0 {
		mem.LastVolSlide = arg
	} else {
		arg = mem.LastVolSlide
	}

	up, down := arg>>4, arg&15
	if down == 15 && up > 0 {
		mem.ITFineVolSlide = up
	} else if up == 15 && down > 0 {
		mem.ITFineVolSlide = down
	}

	if voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	delta := 0

	switch {
	case down == 15 && up > 0:
		if tick == 0 {
			delta = int(up)
		}
	case up == 15 && down > 0:
		if tick == 0 {
			delta = -int(down)
		}
	case tick == 0:
		if down == 15 {
			delta = -15
		} else if up == 15 {
			delta = 15
		}
	case down != 0 && up == 0:
		delta = -int(down)
	case down == 0:
		delta = int(up)
	}

	amp := &w.VoiceAmps[voice]
	amp.Volume = clamp32(amp.Volume+float32(delta)/64, 0, 1)
	amp.itSmoothRamp = delta != 0 || amp.itSmoothRamp
}
