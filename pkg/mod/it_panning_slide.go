// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func applyITPanningSlide(w *Tracker, track TrackID, voice VoiceID, arg uint8) {
	mem := &w.TrackMemories[track].itPanSlide

	if w.Assets.ITCompatibility.LegacyModPlug && arg&15 != 0 && arg&15 != 15 && arg>>4 != 0 && arg>>4 != 15 {
		arg &= 15
	}

	if arg != 0 {
		*mem = arg
	} else {
		arg = *mem
	}

	left, right := int(arg>>4), int(arg&15)
	delta := 0
	tick := w.itRepeatedRowTick()
	switch {
	case right == 15 && left != 0:
		if tick == 0 {
			delta = -left
		}
	case left == 15 && right != 0:
		if tick == 0 {
			delta = right
		}
	case tick != 0 && right == 0:
		delta = -left
	case tick != 0 && left == 0:
		delta = right
	}

	if delta == 0 {
		return
	}

	cursor := &w.TrackCursors[track]
	pan := cursor.Panning

	if voice != -1 && w.VoiceLives[voice].Active {
		pan = w.VoiceAmps[voice].Panning
	}

	pan = clamp32(pan+float32(delta)/32, -1, 1)
	cursor.Panning = pan
	cursor.itHasRestorePan = false

	if voice != -1 && w.VoiceLives[voice].Active {
		w.VoiceAmps[voice].Panning = pan
	}
}
