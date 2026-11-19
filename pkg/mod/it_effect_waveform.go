package mod

import "math"

func itEffectWaveform(w *Tracker, waveform, position uint8, precise, truncateSine bool) int {
	if precise {
		switch waveform & 3 {
		case 1:
			return 64 - (int(position)+1)/2
		case 2:
			if position < 128 {
				return 64
			}
			return 0
		case 3:
			return w.itRandom.Intn(128) - 64
		default:
			return int(math.Round(64 * math.Sin(float64(position)*2*math.Pi/256)))
		}
	}

	pos := int(position & 63)
	switch waveform & 3 {
	case 1:
		if pos < 32 {
			return -pos * 4
		}
		return 255 - pos*4
	case 2:
		if pos < 32 {
			return 127
		}
		return -127
	case 3:
		if w.Assets.s3m {
			return int(s3mRandomWaveform[pos])
		}
		fallthrough
	default:
		sine := 127 * math.Sin(float64(pos)*2*math.Pi/64)
		if truncateSine {
			return int(math.Trunc(sine))
		}
		return int(math.Round(sine))
	}
}

func itSelectWaveform(arg uint8, legacy bool) uint8 {
	waveform := arg & 15
	if legacy {
		return waveform & 7
	}
	if waveform >= 4 {
		return 0
	}
	return waveform
}

func (w *Tracker) applyITWaveformSelection(mem *EffectMemory, cmd ModCommand) {
	if (!w.Assets.IT && !w.Assets.s3m) || w.Clock.Tick != 0 || cmd.EffectCmd != 0x15 {
		return
	}

	arg := cmd.EffectArg
	if arg == 0 {
		arg = mem.ITLastSpecial
	}

	legacy := w.Assets.s3m || w.itLegacyModulationWaveform()

	switch arg & 0xf0 {
	case 0x30:
		mem.itVibratoWaveform = itSelectWaveform(arg, legacy)
		if w.Assets.s3m {
			mem.itVibratoWaveform &= 3
		}
	case 0x40:
		mem.itTremoloWaveform = itSelectWaveform(arg, legacy)
		if w.Assets.s3m {
			mem.itTremoloWaveform &= 3
		}
	case 0x50:
		mem.itPanbrelloWaveform = itSelectWaveform(arg, legacy)
		if w.panbrelloProfile().preciseWaveform {
			mem.itPanbrelloPos = 0
		}
	}
}
