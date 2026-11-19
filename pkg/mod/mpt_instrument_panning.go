package mod

import "fmt"

func applyMPTInstrumentPanning(instruments []ITInstrument, values []uint32) error {
	if values == nil {
		return nil
	}

	if len(values) != len(instruments) {
		return fmt.Errorf("MPTM panning count %d does not match %d instruments", len(values), len(instruments))
	}

	for i, value := range values {
		if value > 256 {
			return fmt.Errorf("MPTM instrument %d uses unsupported extended panning %d", i+1, value)
		}
	}

	for i, value := range values {
		instruments[i].Panning = uint8(value / 4)
		instruments[i].mptPanning = uint16(value)
		instruments[i].hasMPTPanning = true
	}

	return nil
}

func (instrument *ITInstrument) normalizedPanning() float32 {
	if instrument.hasMPTPanning {
		return float32(instrument.mptPanning)/128 - 1
	}
	return float32(instrument.Panning)/32 - 1
}
