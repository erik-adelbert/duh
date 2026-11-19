// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

func (w *Tracker) s3mVibrato(voice VoiceID, mem *EffectMemory, cmd ModCommand) {
	pitch := &w.VoicePitches[voice]
	sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]
	delta := itEffectWaveform(w, mem.itVibratoWaveform, mem.ITVibratoPos, false, false) * int(mem.LastVibrato&15)
	divisor := 16
	if w.Assets.s3mSharedEffectMemory && cmd.EffectCmd == 0x21 ||
		!w.Assets.s3mSharedEffectMemory && mem.itFineVibratoDepth {
		divisor = 64
	}

	delta /= divisor
	speed := sample.C5Speed
	if pitch.itFineTuneSpeed != 0 {
		speed = pitch.itFineTuneSpeed
	}
	frequency := pitch.BaseFreqHz * float64(speed) / 8363
	denominator := 1 + float64(delta)*frequency/(1712*8363)
	if denominator <= 0 {
		w.stopVoice(voice)
		return
	}
	modulated := pitch.BaseFreqHz / denominator
	if w.Assets.s3mFrequencyPitch {
		modulated = math.Max(1, math.Min(math.Floor(frequency/denominator), math.MaxInt32)) * 8363 / float64(speed)
	}
	pitch.VibratoOffset = modulated - pitch.BaseFreqHz
	if w.itRepeatedRowTick() != 0 {
		mem.ITVibratoPos += mem.LastVibrato >> 4
	}
}

func rememberS3MVibrato(mem *EffectMemory, cmd ModCommand, shared bool) {
	arg := cmd.EffectArg
	if arg&0xf0 != 0 {
		mem.LastVibrato = arg&0xf0 | mem.LastVibrato&15
	}
	if arg&15 != 0 {
		mem.LastVibrato = mem.LastVibrato&0xf0 | arg&15
		mem.itFineVibratoDepth = !shared && cmd.EffectCmd == 0x21
	}
}
