// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

func applyMPTCutoffSwing(instruments []ITInstrument, values []uint8) error {
	if err := validateMPTFilterSwing(instruments, values, "cutoff"); err != nil {
		return err
	}
	for i, value := range values {
		instruments[i].mptCutoffSwing = value
	}
	return nil
}

func validateMPTFilterSwing(instruments []ITInstrument, values []uint8, name string) error {
	if values == nil {
		return nil
	}
	if len(values) != len(instruments) {
		return fmt.Errorf("MPTM %s variation count %d does not match %d instruments", name, len(values), len(instruments))
	}
	for i, value := range values {
		if value > 64 {
			return fmt.Errorf("unsupported MPTM instrument %d %s variation %d", i+1, name, value)
		}
	}
	return nil
}

func mptRandomCutoff(cutoff, swing uint8, random int) uint8 {
	return mptApplyFilterSwing(cutoff, mptFilterSwingDelta(cutoff, swing, random))
}

func mptFilterSwingDelta(value, swing uint8, random int) int16 {
	delta := int(swing) * (random + 1) / 128
	return int16((delta*int(value) + 1) / 128)
}

func mptApplyFilterSwing(value uint8, delta int16) uint8 {
	return uint8(max(0, min(127, int(value)+int(delta))))
}

func applyMPTResonanceSwing(instruments []ITInstrument, values []uint8) error {
	if err := validateMPTFilterSwing(instruments, values, "resonance"); err != nil {
		return err
	}
	for i, value := range values {
		instruments[i].mptResonanceSwing = value
	}
	return nil
}
