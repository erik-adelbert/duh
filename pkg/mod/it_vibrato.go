// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

func itSampleVibrato(w *Tracker, pitch *PitchState, sample *SampleAsset) float64 {
	if sample.ITVibratoDepth == 0 || !w.Assets.ITCompatibility.LegacyModPlug && sample.ITVibratoRate == 0 {
		return 1
	}

	depth := uint32(sample.ITVibratoDepth) * 256
	sweep := uint32(sample.ITVibratoSweep)
	if w.Assets.ITCompatibility.LegacyModPlug {
		sweep *= 2
	}

	pitch.ITAutoVibDepth = min(pitch.ITAutoVibDepth+sweep, depth)

	pos := pitch.ITAutoVibPos
	pitch.ITAutoVibPos += sample.ITVibratoRate
	if w.Assets.ITCompatibility.LegacyModPlug {
		pos = pitch.ITAutoVibPos
	}

	var delta int
	switch sample.ITVibratoType {
	case 1:
		delta = 64 - (int(pos)+1)/2

		if w.Assets.ITCompatibility.LegacyModPlug {
			delta = ((64 - int(pos)/2) & 127) - 64
		}
	case 2:
		switch {
		case w.Assets.ITCompatibility.LegacyModPlug:
			delta = -64
			if pos >= 128 {
				delta = 64
			}
		case pos < 128:
			delta = 64
		}
	case 3:
		delta = w.itRandom.Intn(128) - 64
	case 4:
		delta = (int(pos)+1)/2 - 64

		if w.Assets.ITCompatibility.LegacyModPlug {
			delta = ((64 + int(pos)/2) & 127) - 64
		}
	default:
		delta = int(math.Round(64 * math.Sin(float64(pos)*2*math.Pi/256)))

		if w.Assets.ITCompatibility.LegacyModPlug {
			delta = -delta
		}
	}

	if w.Assets.ITCompatibility.LegacyModPlug {
		return math.Pow(2, float64(delta*int(pitch.ITAutoVibDepth)/256)/49152)
	}

	delta = delta * int(pitch.ITAutoVibDepth/256) / 64
	return math.Pow(2, float64(delta)/768)
}
