package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func (e *mptExtensions) readSampleCues(field []byte) error {
	const headerSize, cueSize, maxCues = 2, 4, 9

	if len(field) < headerSize+cueSize || (len(field)-headerSize)%cueSize != 0 || len(field) > headerSize+maxCues*cueSize {
		return fmt.Errorf("invalid size %d for MPTM field %q", len(field), "CUES")
	}

	sample := binary.LittleEndian.Uint16(field)

	if sample == 0 {
		return errors.New("invalid MPTM cue sample index 0")
	}

	if e.sampleCues == nil {
		e.sampleCues = make(map[uint16][]uint32)
	}

	if _, exists := e.sampleCues[sample]; exists {
		return fmt.Errorf("duplicate MPTM cues for sample %d", sample)
	}

	cues := make([]uint32, (len(field)-headerSize)/cueSize)

	for i := range cues {
		cues[i] = binary.LittleEndian.Uint32(field[headerSize+i*cueSize:])
	}

	e.sampleCues[sample] = cues

	return nil
}
