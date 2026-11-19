package mod

import "fmt"

// applyMPTInstrumentFadeouts applies only this extension; it does not authorize
// playback or apply other instrument and song compatibility properties.
func applyMPTInstrumentFadeouts(instruments []ITInstrument, values []uint32) error {
	if values == nil {
		return nil
	}

	if len(values) != len(instruments) {
		return fmt.Errorf("MPTM fadeout count %d does not match %d instruments", len(values), len(instruments))
	}

	for i, value := range values {
		if value > 32768 {
			return fmt.Errorf("unsupported MPTM instrument %d fadeout %d", i+1, value)
		}
	}

	for i, value := range values {
		instruments[i].mptFadeout = uint16(value)
		instruments[i].hasMPTFadeout = true
	}

	return nil
}
