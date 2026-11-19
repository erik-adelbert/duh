package mod

import "math"

func itLegacyNotePitch(note uint8, sample *SampleAsset, linear bool) (float64, int32) {
	periods := [...]uint32{1712, 1616, 1524, 1440, 1356, 1280, 1208, 1140, 1076, 1016, 960, 907}
	octave := note / 12
	base := uint64(periods[note%12]) << 5

	var period uint64

	if linear {
		period = base >> octave
	} else {
		speed := min(uint64(sample.C5Speed), uint64(math.MaxUint32)>>octave)
		period = 8363 * base / (speed << octave)
	}

	period = max(period, 1)
	frequency := float64(1712*8363) / float64(period)

	if !linear {
		frequency *= 8363 / float64(sample.C5Speed)
	}

	return frequency, int32(period)
}

func itGlissandoPitch(pitch *PitchState, sample *SampleAsset, compat ITCompatibility) float64 {
	low, high := 0, 120
	current := *pitch
	current.initITSlidePitch(sample, compat)
	tunedSample := *sample

	if pitch.itFineTuneSpeed != 0 {
		tunedSample.C5Speed = pitch.itFineTuneSpeed
		if compat.LegacyModPlug && compat.LinearSlides {
			current.ITSlidePitch = int32(math.Round(1712 * 8363 * float64(tunedSample.C5Speed) /
				(pitch.BaseFreqHz * float64(sample.C5Speed))))
		}
	}

	for low < high {
		mid := (low + high) / 2

		var below bool

		if compat.LegacyModPlug {
			_, period := itLegacyNotePitch(uint8(mid), &tunedSample, compat.LinearSlides)
			below = period > current.ITSlidePitch
		} else {
			below = itGlissandoFrequency(uint8(mid), tunedSample.C5Speed) < current.ITSlidePitch
		}

		if below {
			low = mid + 1
		} else {
			high = mid
		}
	}

	note := uint8(min(low, 119))

	if compat.LegacyModPlug {
		frequency, _ := itLegacyNotePitch(note, &tunedSample, compat.LinearSlides)
		return frequency * float64(tunedSample.C5Speed) / float64(sample.C5Speed)
	}

	return float64(itGlissandoFrequency(note, tunedSample.C5Speed)) * 8363 / float64(sample.C5Speed)
}

func itGlissandoFrequency(note uint8, c5Speed uint32) int32 {
	table := uint64(math.Round(65536 * math.Pow(2, float64(note%12)/12)))
	frequency := uint64(c5Speed) * (table << (note / 12)) / (65536 << 5)

	return int32(min(frequency, math.MaxInt32))
}

func itPitchSlide(w *Tracker, voice VoiceID, param uint8, direction int) {
	pitch := &w.VoicePitches[voice]
	sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]

	if !itPitchSlideState(pitch, sample, w.Assets.ITCompatibility, param, direction, itPitchSlideTick(w, param)) {
		w.stopVoice(voice)
	}
}

func itPitchSlideTick(w *Tracker, param uint8) int {
	if w.Clock.Speed == 1 && w.Assets.ITCompatibility.mptSlidesAtSpeedOne && param < 0xe0 {
		return 1
	}

	return w.itRepeatedRowTick()
}

func itPitchSlideState(pitch *PitchState, sample *SampleAsset, compat ITCompatibility, param uint8, direction, tick int) bool {
	amount := int(param) * 4
	if param >= 0xe0 {
		amount = int(param & 15)
		if param >= 0xf0 {
			amount *= 4
		}
		if tick != 0 {
			return true
		}
	} else if tick == 0 {
		return true
	}

	if amount == 0 {
		return true
	}

	return applyITPitchSlideState(pitch, sample, compat, amount, direction, param >= 0xe0)
}

func itVolumePitchSlideState(pitch *PitchState, sample *SampleAsset, compat ITCompatibility, param uint8, direction, tick int) bool {
	if !compat.itVolColFinePortamento {
		return itPitchSlideState(pitch, sample, compat, param, direction, tick)
	}

	if tick == 0 || param == 0 {
		return true
	}

	return applyITPitchSlideState(pitch, sample, compat, int(param)*4, direction, false)
}

func applyITPitchSlide(w *Tracker, voice VoiceID, amount, direction int, fine bool) {
	pitch := &w.VoicePitches[voice]
	sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]

	if !applyITPitchSlideState(pitch, sample, w.Assets.ITCompatibility, amount, direction, fine) {
		w.stopVoice(voice)
	}
}

func applyITPitchSlideState(pitch *PitchState, sample *SampleAsset, compat ITCompatibility, amount, direction int, fine bool) bool {
	pitch.initITSlidePitch(sample, compat)
	old := pitch.ITSlidePitch

	var next int32
	if compat.LinearSlides {
		sign := direction
		if compat.LegacyModPlug {
			sign = -sign
		}
		factor := math.Round(65536 * math.Pow(2, float64(sign*amount)/768))
		next = int32(math.Min(math.Round(float64(old)*factor/65536), math.MaxInt32))
		if next == old {
			next += int32(sign)
		}
	} else if compat.LegacyModPlug {
		next = old - int32(direction*amount)
		if direction < 0 && fine {
			next = min(next, 65535)
		}
	} else {
		denominator := float64(1712*8363) - float64(direction*amount)*float64(old)
		if denominator <= 0 {
			return false
		}
		next = int32(math.Min(float64(1712*8363)*float64(old)/denominator, math.MaxInt32))
	}

	pitch.setITSlidePitch(next, compat)

	return true
}

func (pitch *PitchState) initITSlidePitch(sample *SampleAsset, compat ITCompatibility) {
	if pitch.ITSlidePitch != 0 {
		return
	}

	frequency := pitch.BaseFreqHz * float64(sample.C5Speed) / 8363

	if compat.LegacyModPlug {
		if compat.LinearSlides {
			pitch.ITSlidePitch = int32(math.Round(1712 * 8363 / pitch.BaseFreqHz))
		} else {
			pitch.ITSlidePitch = int32(math.Round(1712 * 8363 / frequency))
		}
	} else {
		pitch.ITSlidePitch = int32(math.Min(frequency, math.MaxInt32))
	}

	pitch.ITSlidePitch = max(1, pitch.ITSlidePitch)
}

func (pitch *PitchState) setITSlidePitch(next int32, compat ITCompatibility) {
	old := pitch.ITSlidePitch
	pitch.ITSlidePitch = max(1, next)

	if compat.LegacyModPlug {
		pitch.BaseFreqHz *= float64(old) / float64(pitch.ITSlidePitch)
	} else {
		pitch.BaseFreqHz *= float64(pitch.ITSlidePitch) / float64(old)
	}
}

func itTonePortamento(w *Tracker, track TrackID, voice VoiceID, cmd ModCommand, speed uint8, startTick int) {
	pitch := &w.VoicePitches[voice]
	pitch.itPortamento = true

	if w.Clock.Tick == startTick && cmd.Note >= 1 && cmd.Note <= 120 {
		note := cmd.Note

		if len(w.Assets.ITInstruments) > 0 {
			instrument := w.TrackMemories[track].LastInstrument
			if instrument == 0 || int(instrument) > len(w.Assets.ITInstruments) {
				return
			}
			mapping := &w.Assets.ITInstruments[instrument-1]
			if mapping.Samples[cmd.Note-1] == 0 {
				return
			}
			note = mapping.Notes[cmd.Note-1] + 1
		}

		pitch.PortaTargetHz = noteToHz(note - 1)

		if w.Assets.ITCompatibility.LegacyModPlug {
			sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]
			pitch.PortaTargetHz, _ = itLegacyNotePitch(note-1, sample, w.Assets.ITCompatibility.LinearSlides)
			if w.Assets.ITCompatibility.LinearSlides && pitch.itFineTuneSpeed != 0 {
				pitch.PortaTargetHz *= float64(pitch.itFineTuneSpeed) / float64(sample.C5Speed)
			}
		}
	}

	slideAtSpeedOne := startTick == 0 && w.Clock.Speed == 1 && w.Assets.ITCompatibility.mptSlidesAtSpeedOne
	tick := w.itRepeatedRowTick()
	firstTick := tick == startTick && !slideAtSpeedOne && (startTick == 0 || !w.Assets.ITCompatibility.LegacyModPlug)

	if tick < startTick || firstTick || speed == 0 || pitch.PortaTargetHz <= 0 {
		return
	}

	target := pitch.PortaTargetHz
	direction := 1

	if pitch.BaseFreqHz > target {
		direction = -1
	} else if pitch.BaseFreqHz == target {
		return
	}

	pitch.initITSlidePitch(&w.Assets.Samples[w.VoiceSamples[voice].SampleID], w.Assets.ITCompatibility)

	if direction > 0 && !w.Assets.ITCompatibility.LinearSlides && !w.Assets.ITCompatibility.LegacyModPlug &&
		int64(speed)*4*int64(pitch.ITSlidePitch) >= 1712*8363 {
		pitch.BaseFreqHz = target
	} else {
		applyITPitchSlide(w, voice, int(speed)*4, direction, false)
	}

	if direction > 0 && pitch.BaseFreqHz >= target || direction < 0 && pitch.BaseFreqHz <= target {
		pitch.BaseFreqHz = target
		pitch.ITSlidePitch = 0
		if !w.Assets.ITCompatibility.LegacyModPlug {
			pitch.PortaTargetHz = 0
		}
	}
}
