// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

func applyMPTInstrumentGlobalVolume(instruments []ITInstrument, values []uint32) error {
	if values == nil {
		return nil
	}

	if len(values) != len(instruments) {
		return fmt.Errorf("MPTM instrument global volume count %d does not match %d instruments", len(values), len(instruments))
	}

	for i, value := range values {
		if value > 65535 {
			return fmt.Errorf("MPTM instrument %d uses unsupported extended global volume %d", i+1, value)
		}
	}

	for i, value := range values {
		instruments[i].GlobalVolume = uint8(min(value, 64) * 2)
		instruments[i].mptGlobalVolumeOverride = true
	}

	return nil
}

func itInstrumentSampleVolume(sample *SampleAsset, instrument *ITInstrument) float32 {
	if instrument.mptGlobalVolumeOverride {
		return float32(int(sample.ITGlobalVolume)*int(instrument.GlobalVolume/2)/64) / 64
	}
	return float32(sample.ITGlobalVolume) * float32(instrument.GlobalVolume) / (64 * 128)
}
