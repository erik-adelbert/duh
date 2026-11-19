// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) applyITTremolo(track TrackID, voice VoiceID, cmd ModCommand) {
	active := voice != -1 && w.VoiceLives[voice].Active
	if active {
		w.VoiceAmps[voice].itTremoloOffset = 0
	}

	if cmd.EffectCmd != 0x23 {
		return
	}

	mem := &w.TrackMemories[track]
	if cmd.EffectArg&0xf0 != 0 {
		mem.itTremolo = mem.itTremolo&15 | cmd.EffectArg&0xf0
	}

	if cmd.EffectArg&15 != 0 {
		mem.itTremolo = mem.itTremolo&0xf0 | cmd.EffectArg&15
	}

	if !active {
		return
	}

	legacyWaveform := w.Assets.s3m || w.itLegacyModulationWaveform()
	delta := itEffectWaveform(w, mem.itTremoloWaveform, mem.itTremoloPos, !legacyWaveform, false)
	divisor := 32

	if legacyWaveform {
		divisor = 64
	}

	if !legacyWaveform || w.VoiceAmps[voice].Volume > 0 {
		w.VoiceAmps[voice].itTremoloOffset = float32(delta*int(mem.itTremolo&15)*4/divisor) / 256
	}

	if w.Clock.Tick != 0 || !w.Assets.ITCompatibility.OldEffects && !w.Assets.s3m {
		speed := mem.itTremolo >> 4
		if !legacyWaveform {
			speed *= 4
		}
		mem.itTremoloPos += speed
	}
}
