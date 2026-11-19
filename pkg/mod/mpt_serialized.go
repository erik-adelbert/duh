package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// parseMPTSerializedBlock accepts only the indexed SSB layout emitted by
// OpenMPT for the target. It does not interpret or authorize its entries.
func parseMPTSerializedBlock(data []byte, object string) (map[string][]byte, error) {
	fail := func(reason string) (map[string][]byte, error) {
		return nil, fmt.Errorf("MPTM %s: %s", object, reason)
	}

	if len(data) < 4 || string(data[:3]) != "228" || int(data[3]) != len(object) ||
		len(data) < 4+len(object) || string(data[4:4+len(object)]) != object {
		return fail("invalid object header")
	}

	pos := 4 + len(object)
	if pos >= len(data) || data[pos] != 0x1f {
		return fail("unsupported serialization layout")
	}
	pos++

	headerSize, err := mptAdaptive(data, &pos, 2, false)
	if err != nil || headerSize != 2 || len(data)-pos < 2 || data[pos] != 0 || data[pos+1] != 1 {
		return fail("unsupported or truncated serialization options")
	}

	pos += 2
	if _, err := mptAdaptive(data, &pos, 2, true); err != nil {
		return fail("truncated object version")
	}
	if pos >= len(data) || data[pos] != 1 {
		return fail("unsupported entry identifier format")
	}

	pos++
	count, err := mptAdaptive(data, &pos, 2, true)
	if err != nil || count > 16000 {
		return fail("invalid entry count")
	}

	mapOffset, err := mptAdaptive(data, &pos, 2, true)
	if err != nil || mapOffset < uint64(pos) || mapOffset > uint64(len(data)) {
		return fail("invalid entry map offset")
	}

	mapPos := int(mapOffset)
	type entry struct {
		id         string
		start, end int
	}

	entries := make([]entry, 0, int(count))
	seen := make(map[string]bool)
	for range count {
		size, err := mptAdaptive(data, &mapPos, 1, false)
		if err != nil || size > uint64(len(data)-mapPos) {
			return fail("truncated entry identifier")
		}

		id := string(data[mapPos : mapPos+int(size)])
		mapPos += int(size)
		start, err := mptAdaptive(data, &mapPos, 2, true)
		if err != nil {
			return fail("truncated entry offset")
		}

		length, err := mptAdaptive(data, &mapPos, 2, true)
		if err != nil || start < uint64(pos) || start > mapOffset || length > mapOffset-start {
			return fail("invalid entry bounds")
		}

		if seen[id] {
			return fail("duplicate entry identifier")
		}

		seen[id] = true
		entries = append(entries, entry{id, int(start), int(start + length)})
	}

	if mapPos != len(data) {
		return fail("trailing or truncated entry map")
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].start < entries[j].start })

	result := make(map[string][]byte, len(entries))
	end := pos
	for _, entry := range entries {
		if entry.start != end {
			return fail("overlapping or unaccounted entry data")
		}
		result[entry.id] = data[entry.start:entry.end]
		end = entry.end
	}

	if end != int(mapOffset) {
		return fail("unaccounted entry data")
	}

	return result, nil
}

func mptAdaptive(data []byte, pos *int, tagBits int, exponential bool) (uint64, error) {
	if *pos >= len(data) {
		return 0, errors.New("truncated adaptive integer")
	}

	first := data[*pos]
	length := 1 + int(first&byte((1<<tagBits)-1))

	if exponential {
		length = 1 << (first & 3)
	}

	if length > len(data)-*pos {
		return 0, errors.New("truncated adaptive integer")
	}

	var value uint64
	for i := range length {
		value |= uint64(data[*pos+i]) << (8 * i)
	}

	*pos += length
	return value >> tagBits, nil
}

func mptUint16(data []byte) (uint16, error) {
	if len(data) != 2 {
		return 0, errors.New("expected 16-bit value")
	}
	return binary.LittleEndian.Uint16(data), nil
}
