package mod

import "math"

var s3mNotePeriods = [...]uint64{1712, 1616, 1524, 1440, 1356, 1280, 1208, 1140, 1076, 1016, 960, 907}
var s3mFrequencyMultipliers = [...]uint64{65536, 69433, 73562, 77936, 82570, 87480, 92682, 98193, 104032, 110218, 116772, 123715}

func s3mNoteFrequency(note uint8, speed uint32, frequencyPitch bool) float64 {
	if speed == 0 {
		speed = 8363
	}
	octave := note / 12
	if frequencyPitch {
		frequency := uint64(speed) * (s3mFrequencyMultipliers[note%12] << octave) / (65536 << 5)
		return float64(frequency) * 8363 / float64(speed)
	}
	period := uint64(8363) * (s3mNotePeriods[note%12] << 5) / (uint64(speed) << octave)
	if period == 0 {
		period = 1
	}
	return 1712.0 * 8363 * 8363 / (float64(period) * float64(speed))
}

func s3mGlissandoFrequency(frequency float64, speed uint32, frequencyPitch bool) float64 {
	low, high := 0, 120
	for low < high {
		mid := (low + high) / 2
		if s3mNoteFrequency(uint8(mid), speed, frequencyPitch) < frequency-1e-8 {
			low = mid + 1
		} else {
			high = mid
		}
	}
	return s3mNoteFrequency(uint8(min(low, 119)), speed, frequencyPitch)
}

func (w *Tracker) s3mPitchSlide(voice VoiceID, param uint8, direction int) {
	tick := w.itRepeatedRowTick()
	amount := int(param) * 4
	if param >= 0xe0 {
		amount = int(param & 15)
		if param >= 0xf0 {
			amount *= 4
		}
		if tick != 0 {
			return
		}
	} else if tick == 0 {
		return
	}
	pitch := &w.VoicePitches[voice]
	speed := w.Assets.Samples[w.VoiceSamples[voice].SampleID].C5Speed
	if pitch.itFineTuneSpeed != 0 {
		speed = pitch.itFineTuneSpeed
	}
	if speed == 0 || pitch.BaseFreqHz <= 0 {
		return
	}
	if w.Assets.s3mFrequencyPitch {
		frequency := math.Round(pitch.BaseFreqHz * float64(speed) / 8363)
		denominator := 1712*8363 - float64(amount*direction)*frequency
		if denominator <= 0 {
			w.stopVoice(voice)
			return
		}
		frequency = math.Min(math.Floor(1712*8363*frequency/denominator), math.MaxInt32)
		pitch.BaseFreqHz = math.Max(1, frequency) * 8363 / float64(speed)
		return
	}
	period := 1712*8363*8363/(pitch.BaseFreqHz*float64(speed)) - float64(amount*direction)
	if period <= 0 {
		w.stopVoice(voice)
		return
	}
	pitch.BaseFreqHz = 1712 * 8363 * 8363 / (period * float64(speed))
}
