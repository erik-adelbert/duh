// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "fmt"

func (e *mptExtensions) validatePlaybackFields(sampleCount int) error {
	if err := validateMPTVolumeRamp(e.instrumentVolumeRamp); err != nil {
		return err
	}

	for i, mode := range e.instrumentPluginVelocity {
		if mode > 1 {
			return fmt.Errorf("MPTM instrument %d uses unsupported plugin velocity handling %d", i+1, mode)
		}
	}

	for i, mode := range e.instrumentPluginVolume {
		if mode > 3 {
			return fmt.Errorf("MPTM instrument %d uses unsupported plugin volume handling %d", i+1, mode)
		}
	}

	for sample := range e.sampleCues {
		if int(sample) > sampleCount {
			return fmt.Errorf("MPTM cues reference nonexistent sample %d", sample)
		}
	}

	for _, field := range []struct {
		name   string
		values []uint8
		unset  uint8
	}{
		{"extended MIDI channel", e.instrumentMIDIChannel, 0},
		{"extended MIDI program", e.instrumentMIDIProgram, 0},
	} {
		if err := validateMPTInstrumentField(field.name, field.values, field.unset); err != nil {
			return err
		}
	}

	if err := validateMPTInstrumentField("extended MIDI bank", e.instrumentMIDIBank, uint16(0)); err != nil {
		return err
	}

	for i, mode := range e.instrumentFilterMode {
		if mode != 0xff && mode > 1 {
			return fmt.Errorf("MPTM instrument %d uses unsupported filter mode %d", i+1, mode)
		}
	}

	for _, field := range []struct {
		name   string
		values []uint8
		unset  uint8
	}{
		{"pitch-envelope release node", e.instrumentPitchReleaseNode, 0xff},
		{"pan-envelope release node", e.instrumentPanReleaseNode, 0xff},
	} {
		if err := validateMPTInstrumentField(field.name, field.values, field.unset); err != nil {
			return err
		}
	}

	if err := validateMPTInstrumentField("pitch-to-tempo lock", e.instrumentPitchTempoLock, uint16(0)); err != nil {
		return err
	}
	return validateMPTInstrumentField("fractional pitch-to-tempo lock", e.instrumentPitchTempoFraction, uint16(0))
}

func validateMPTInstrumentField[T ~uint8 | ~uint16](name string, values []T, unset T) error {
	for i, value := range values {
		if value != unset {
			return fmt.Errorf("MPTM instrument %d uses unsupported %s %d", i+1, name, value)
		}
	}
	return nil
}
