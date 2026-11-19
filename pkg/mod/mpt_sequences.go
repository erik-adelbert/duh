// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"
)

type mptSequence struct {
	name         string
	utf8Name     bool
	orders       []uint16
	restart      *uint16
	defaultTempo *uint32 // OpenMPT's raw tempo, in ten-thousandths of a BPM.
	defaultSpeed *uint32
}

type mptSequenceSet struct {
	current   uint8
	sequences []mptSequence
	patterns  []byte
}

// parseMPTSequences preserves sequence data but does not select or play it.
func parseMPTSequences(data []byte, patternCount int) (mptSequenceSet, error) {
	var result mptSequenceSet

	if patternCount < 1 || patternCount > 65534 {
		return result, errors.New("invalid MPTM pattern count")
	}

	root, err := parseMPTSerializedBlock(data, "mptm")
	if err != nil {
		return result, err
	}

	if len(root) != 1 && len(root) != 2 || root["mptSeqC"] == nil || len(root) == 2 && root["mptPc"] == nil {
		return result, errors.New("unsupported MPTM serialized properties")
	}

	result.patterns = root["mptPc"]
	set, err := parseMPTSerializedBlock(root["mptSeqC"], "mptSeqC")
	if err != nil {
		return result, err
	}

	count, current := set["n"], set["c"]
	if len(count) != 1 || count[0] == 0 || count[0] > 50 || len(current) != 1 || current[0] >= count[0] || len(set) != int(count[0])+2 {
		return result, errors.New("invalid MPTM sequence count or selection")
	}

	result.current = current[0]
	result.sequences = make([]mptSequence, count[0])
	for i := range result.sequences {
		block, ok := set[string([]byte{byte(i)})]
		if !ok {
			return mptSequenceSet{}, fmt.Errorf("missing MPTM sequence %d", i)
		}

		seq, err := parseMPTSequence(block, patternCount)
		if err != nil {
			return mptSequenceSet{}, fmt.Errorf("MPTM sequence %d: %w", i, err)
		}

		result.sequences[i] = seq
	}
	return result, nil
}

func parseMPTSequence(data []byte, patternCount int) (mptSequence, error) {
	var seq mptSequence

	fields, err := parseMPTSerializedBlock(data, "mptSeq")
	if err != nil {
		return seq, err
	}

	for key := range fields {
		switch key {
		case "u", "n", "l", "a", "r", "t", "s":
		default:
			return seq, fmt.Errorf("unsupported sequence field %q", key)
		}
	}

	orderBytes, present := fields["a"]
	length, err := mptUint16(fields["l"])
	if err != nil || !present || len(orderBytes) != int(length)*2 {
		return seq, errors.New("invalid sequence length or orders")
	}

	seq.orders = make([]uint16, length)
	for i := range seq.orders {
		order := binary.LittleEndian.Uint16(orderBytes[i*2:])
		if order < 0xfffe && int(order) >= patternCount {
			return mptSequence{}, fmt.Errorf("order %d references missing pattern %d", i, order)
		}
		seq.orders[i] = order
	}

	if value, ok := fields["u"]; ok {
		if len(value) != 1 || value[0] > 1 {
			return mptSequence{}, errors.New("invalid sequence name encoding")
		}
		seq.utf8Name = value[0] == 1
	}

	if value, ok := fields["n"]; ok {
		if len(value) < 1 {
			return mptSequence{}, errors.New("truncated sequence name")
		}

		sizeBytes := 1 + int((value[0]>>2)&3)
		if len(value) < sizeBytes || value[0]&3 != 0 {
			return mptSequence{}, errors.New("unsupported sequence name encoding")
		}

		var header uint32
		for i := range sizeBytes {
			header |= uint32(value[i]) << (8 * i)
		}

		if uint64(header>>4) != uint64(len(value)-sizeBytes) || seq.utf8Name && !utf8.Valid(value[sizeBytes:]) {
			return mptSequence{}, errors.New("invalid sequence name")
		}
		seq.name = string(value[sizeBytes:])
	}

	if value, ok := fields["r"]; ok {
		restart, err := mptUint16(value)
		if err != nil || restart >= length {
			return mptSequence{}, errors.New("invalid sequence restart position")
		}

		seq.restart = &restart
	}

	for _, field := range []struct {
		key string
		dst **uint32
	}{{"t", &seq.defaultTempo}, {"s", &seq.defaultSpeed}} {
		if value, ok := fields[field.key]; ok {
			if len(value) != 4 {
				return mptSequence{}, fmt.Errorf("invalid sequence field %q", field.key)
			}

			n := binary.LittleEndian.Uint32(value)
			*field.dst = &n
		}
	}

	return seq, nil
}
