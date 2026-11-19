// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
)

func applyMPTPatternOverrides(data []byte, patterns []PatternAsset, channels int) (bool, error) {
	fields, err := parseMPTSerializedBlock(data, "mptPc")
	if err != nil {
		return false, err
	}

	count, err := mptUint16(fields["num"])
	if err != nil || int(count) > len(patterns) {
		return false, errors.New("invalid MPTM serialized pattern count")
	}

	bypassed := false
	for id, block := range fields {
		if id == "num" {
			continue
		}

		if len(id) != 2 {
			return false, errors.New("invalid MPTM serialized pattern identifier")
		}

		index := int(binary.LittleEndian.Uint16([]byte(id)))
		if index >= int(count) {
			return false, fmt.Errorf("MPTM serialized pattern %d exceeds pattern count", index)
		}

		pattern, err := parseMPTSerializedBlock(block, "mptP")
		if err != nil {
			return false, fmt.Errorf("MPTM pattern %d: %w", index, err)
		}

		var (
			rowsPerBeat, rowsPerMeasure       uint32
			hasRowsPerBeat, hasRowsPerMeasure bool
			swingData                         []byte
			hasSwing                          bool
		)
		for key, value := range pattern {
			switch key {
			case "data":
				ignored, err := applyMPTPatternData(value, &patterns[index], channels)
				if err != nil {
					return false, fmt.Errorf("MPTM pattern %d: %w", index, err)
				}
				bypassed = bypassed || ignored
			case "COL":
				if len(value) != 4 {
					return false, fmt.Errorf("MPTM pattern %d: invalid color", index)
				}
			case "RPB.", "RPM.":
				if len(value) != 4 {
					return false, fmt.Errorf("MPTM pattern %d: invalid %s signature field", index, key)
				}
				if key == "RPB." {
					rowsPerBeat, hasRowsPerBeat = binary.LittleEndian.Uint32(value), true
				} else {
					rowsPerMeasure, hasRowsPerMeasure = binary.LittleEndian.Uint32(value), true
				}
			case "SWNG":
				swingData, hasSwing = value, true
			default:
				return false, fmt.Errorf("MPTM pattern %d: unsupported serialized pattern field %q", index, key)
			}
		}

		if hasRowsPerBeat || hasRowsPerMeasure {
			if !hasRowsPerBeat || !hasRowsPerMeasure || rowsPerBeat == 0 || rowsPerBeat > rowsPerMeasure || rowsPerMeasure > 65536 {
				return false, fmt.Errorf("MPTM pattern %d: invalid pattern signature (%d rows per beat, %d rows per measure)", index, rowsPerBeat, rowsPerMeasure)
			}
			patterns[index].mptRowsPerBeat, patterns[index].mptRowsPerMeasure = rowsPerBeat, rowsPerMeasure
		}

		if hasSwing {
			swing, err := mptPatternTempoSwing(swingData, rowsPerBeat)
			if err != nil {
				return false, fmt.Errorf("MPTM pattern %d: %w", index, err)
			}
			patterns[index].mptTempoSwing = swing
		}
	}
	return bypassed, nil
}

func mptPatternTempoSwing(data []byte, rowsPerBeat uint32) ([]uint32, error) {
	if len(data) < 2 || len(data) != 2+int(binary.LittleEndian.Uint16(data))*4 {
		return nil, errors.New("invalid pattern tempo swing size")
	}

	count := int(binary.LittleEndian.Uint16(data))
	swing := make([]uint32, count)
	for i := range swing {
		swing[i] = binary.LittleEndian.Uint32(data[2+i*4:])
	}

	normalizeMPTTempoSwing(swing)
	if len(swing) == 0 {
		return nil, nil
	}

	swing = resizeMPTTempoSwing(swing, int(rowsPerBeat))
	normalizeMPTTempoSwing(swing)

	return swing, nil
}

func resizeMPTTempoSwing(swing []uint32, rowsPerBeat int) []uint32 {
	if rowsPerBeat < len(swing) {
		swing = swing[:rowsPerBeat]
	} else {
		for len(swing) < rowsPerBeat {
			swing = append(swing, 1<<24)
		}
	}
	normalizeMPTTempoSwing(swing)
	return swing
}

func normalizeMPTTempoSwing(swing []uint32) {
	if len(swing) == 0 {
		return
	}
	const unity = uint32(1 << 24)

	var sum uint64
	for i, value := range swing {
		swing[i] = max(unity/4, min(unity*4, value))
		sum += uint64(swing[i])
	}

	average := sum / uint64(len(swing))
	remaining := int64(unity) * int64(len(swing))
	for i, value := range swing {
		swing[i] = uint32((uint64(value)*uint64(unity) + average/2) / average)
		remaining -= int64(swing[i])
	}
	swing[0] = uint32(int64(swing[0]) + remaining)
}

func applyMPTPatternData(data []byte, pattern *PatternAsset, channels int) (bool, error) {
	var last [MaxTracks][6]byte

	pos, row := 0, 0
	bypassed := false
	for row < len(pattern.Rows) {
		if pos >= len(data) {
			return false, errors.New("truncated serialized pattern data")
		}

		command := data[pos]
		pos++
		if command == 0 {
			row++
			continue
		}

		channel := int(command&0x7f) - 1
		if channel < 0 || channel >= channels || channel >= MaxTracks || channel >= len(pattern.Rows[row]) {
			return false, fmt.Errorf("invalid serialized pattern channel %d", channel+1)
		}

		mask := byte(0)
		if command&0x80 != 0 {
			if pos >= len(data) {
				return false, errors.New("truncated serialized pattern mask")
			}
			mask = data[pos]
			pos++
		}

		if mask&0x80 != 0 {
			return false, errors.New("unsupported serialized pattern mask")
		}

		for i := range 6 {
			if mask&(1<<i) != 0 {
				if pos >= len(data) {
					return false, errors.New("truncated serialized pattern command")
				}
				last[channel][i] = data[pos]
				pos++
			}
		}

		if mask&0x40 != 0 {
			if pos >= len(data) || int(data[pos]) > len(data)-pos-1 {
				return false, errors.New("truncated serialized pattern extra data")
			}
			pos += 1 + int(data[pos])
		}

		if last[channel][0] != 0xfb && last[channel][0] != 0xfc {
			return false, errors.New("unsupported serialized pattern command")
		}

		pattern.Rows[row][channel] = ModCommand{}
		bypassed = true
	}
	if pos != len(data) {
		return false, errors.New("trailing serialized pattern data")
	}
	return bypassed, nil
}
