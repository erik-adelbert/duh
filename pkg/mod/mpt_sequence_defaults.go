package mod

import (
	"errors"
	"fmt"
)

const mptTempoScale = 10000

// mptSequenceDefaults resolves metadata only; it does not select a subsong or
// establish restart and end-of-song playback behavior.
type mptSequenceDefaults struct {
	restart uint16
	tempo   uint32 // ten-thousandths of a BPM
	speed   uint32
}

func mptLegacySequenceDefaults(headerTempo uint8, headerSpeed uint8, extendedTempo, fraction *uint32) (mptSequenceDefaults, error) {
	tempo := uint32(headerTempo)
	if extendedTempo != nil {
		tempo = *extendedTempo
	}

	if tempo < 32 || tempo > 1000 {
		return mptSequenceDefaults{}, fmt.Errorf("unsupported MPTM default tempo %d", tempo)
	}

	if fraction != nil {
		if *fraction >= mptTempoScale {
			return mptSequenceDefaults{}, fmt.Errorf("unsupported MPTM default tempo fraction %d", *fraction)
		}
	}

	tempo *= mptTempoScale
	if fraction != nil {
		tempo += *fraction
	}

	return mptSequenceDefaults{tempo: tempo, speed: uint32(headerSpeed)}, nil
}

func resolveMPTSequenceDefaults(seq mptSequence, legacy mptSequenceDefaults) (mptSequenceDefaults, error) {
	result := legacy

	if seq.restart != nil {
		result.restart = *seq.restart
	}

	if seq.defaultTempo != nil && *seq.defaultTempo != 0 {
		result.tempo = *seq.defaultTempo
	}

	if seq.defaultSpeed != nil && *seq.defaultSpeed != 0 {
		result.speed = *seq.defaultSpeed
	}

	if len(seq.orders) == 0 || int(result.restart) >= len(seq.orders) {
		return mptSequenceDefaults{}, errors.New("invalid effective MPTM restart position")
	}

	if result.tempo < 32*mptTempoScale || result.tempo > 1000*mptTempoScale {
		return mptSequenceDefaults{}, fmt.Errorf("unsupported MPTM sequence tempo %d", result.tempo)
	}

	if result.speed < 1 || result.speed > 255 {
		return mptSequenceDefaults{}, fmt.Errorf("unsupported MPTM sequence speed %d", result.speed)
	}

	return result, nil
}
