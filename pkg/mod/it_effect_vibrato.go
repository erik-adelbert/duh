// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

func (w *Tracker) itLegacyModulationWaveform() bool {
	compat := w.Assets.ITCompatibility
	return compat.LegacyModPlug || compat.mptMixProfile != nil && !w.panbrelloProfile().preciseWaveform
}

func itVibrato(w *Tracker, voice VoiceID, mem *EffectMemory) {
	compat := w.Assets.ITCompatibility
	legacyWaveform := w.itLegacyModulationWaveform()
	advance := w.itRepeatedRowTick() != 0 || !compat.OldEffects
	speed, depth := mem.LastVibrato>>4, int(mem.LastVibrato&15)*4

	if mem.itFineVibratoDepth {
		depth /= 4
	}

	if !legacyWaveform && advance {
		mem.ITVibratoPos += speed * 4
	}

	delta := itEffectWaveform(w, mem.itVibratoWaveform, mem.ITVibratoPos, !legacyWaveform, false)

	if legacyWaveform {
		divisor := 128
		if compat.OldEffects {
			divisor = 64
		}
		delta = -delta * depth / divisor
	} else {
		divisor := 64
		if compat.OldEffects {
			divisor = 32
			delta = -delta
		}
		delta = delta * depth / divisor
	}

	if legacyWaveform && advance {
		mem.ITVibratoPos += speed
	}

	if delta == 0 {
		return
	}

	pitch := &w.VoicePitches[voice]
	sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]
	pitch.itVibratoDelta = delta

	offset, ok := itVibratoOffset(*pitch, sample, compat, delta)
	if !ok {
		w.stopVoice(voice)
		return
	}

	pitch.VibratoOffset = offset
}

func itVibratoOffset(temp PitchState, sample *SampleAsset, compat ITCompatibility, delta int) (float64, bool) {
	base := temp.BaseFreqHz
	temp.initITSlidePitch(sample, compat)
	old := temp.ITSlidePitch
	next := old
	switch {
	case compat.LinearSlides:
		amount := delta
		if amount >= 16 || amount <= -16 {
			amount = amount / 4 * 4
		}
		if compat.LegacyModPlug {
			amount = -amount
		}
		factor := math.Round(65536 * math.Pow(2, float64(amount)/768))
		next = int32(math.Min(math.Round(float64(old)*factor/65536), math.MaxInt32))
		if next == old {
			if amount > 0 {
				next++
			} else {
				next--
			}
		}
	case compat.LegacyModPlug:
		next -= int32(delta)
	default:
		denominator := float64(1712*8363) - float64(delta)*float64(old)
		if denominator <= 0 {
			return 0, false
		}
		next = int32(math.Min(float64(1712*8363)*float64(old)/denominator, math.MaxInt32))
	}
	temp.setITSlidePitch(next, compat)
	return temp.BaseFreqHz - base, true
}
