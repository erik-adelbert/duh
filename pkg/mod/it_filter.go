package mod

import "math"

func newITFilter(cutoff, resonance uint8, sampleRate float64, instrument, legacy, extended bool) ITFilterState {
	if !instrument || cutoff == 127 && (legacy || resonance == 0) {
		return ITFilterState{}
	}
	return itFilterCoefficients(cutoff, resonance, sampleRate, legacy, extended, 256)
}

func itFilterCoefficients(cutoff, resonance uint8, sampleRate float64, legacy, extended bool, modifier int) ITFilterState {
	filterRange := 24.0
	if extended {
		filterRange = 20
	}

	frequency := 110 * math.Pow(2, .25+float64(cutoff)*float64(modifier+256)/(512*filterRange))
	frequency = math.Min(math.Max(frequency, 120), math.Min(20000, sampleRate/2))

	fc := frequency * 2 * math.Pi
	damping := math.Pow(10, -float64(resonance)*24/(128*20))

	var d, e float64

	if !legacy && !extended {
		r := sampleRate / fc
		d, e = damping*r+damping-1, r*r
	} else {
		r := fc / sampleRate
		d, e = (2*damping-math.Min((1-2*damping)*r, 2))/r, 1/(r*r)
	}

	denominator := 1 + d + e

	return ITFilterState{
		Enabled: true,
		A0:      1 / denominator,
		B0:      (d + 2*e) / denominator,
		B1:      -e / denominator,
	}
}

func setITFilterMode(filter *ITFilterState, highPass bool) {
	filter.HighPass = highPass
	if filter.Enabled && highPass {
		filter.A0 = 1 - filter.A0
	}
}

func updateITFilterEnvelope(w *Tracker, voice int, value float32) {
	life := &w.VoiceLives[voice]
	modifier := int(math.Floor(float64(value+32)*8+0.5)) - 256
	cutoff, resonance := life.ITFilterCutoff, life.ITFilterResonance
	left, right := &w.VoiceFilters[voice], &w.VoiceRightFilters[voice]
	computedCutoff := int(cutoff) * (modifier + 256) / 256

	if !w.Assets.ITCompatibility.LegacyModPlug && resonance == 0 && computedCutoff >= 254 {
		return
	}

	if channel := w.VoiceSamples[voice].oplChannel; channel != 0 && w.Assets.ITCompatibility.mptMixProfile != nil {
		w.opl.setModulatorCutoff(channel-1, uint8(computedCutoff/4))
	}

	filter := itFilterCoefficients(cutoff, resonance, w.Clock.SampleRate,
		w.Assets.ITCompatibility.LegacyModPlug, w.Assets.ITCompatibility.ExtendedFilter, modifier)
	setITFilterMode(&filter, life.mptHighPass)

	if left.Enabled {
		filter.Y1, filter.Y2 = left.Y1, left.Y2
	}
	*left = filter

	if right.Enabled {
		filter.Y1, filter.Y2 = right.Y1, right.Y2
	} else {
		filter.Y1, filter.Y2 = 0, 0
	}
	*right = filter
}
