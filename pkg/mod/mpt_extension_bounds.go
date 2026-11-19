// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

type mptExtensionBounds struct {
	propertiesStart int
	serializedStart int
	serializedEnd   int
}

// locateMPTExtensions finds extension bounds for the supported MPTM layout.
// It does not interpret properties or establish playback compatibility.
func locateMPTExtensions(data []byte) (mptExtensionBounds, error) {
	var bounds mptExtensionBounds
	if !isMPTM(data) || string(data[:4]) != "IMPM" {
		return bounds, errors.New("not a supported MPTM header")
	}

	end := int(binary.LittleEndian.Uint32(data[len(data)-4:]))

	u16 := func(off int) int { return int(binary.LittleEndian.Uint16(data[off:])) }

	orders, instruments, samples, patterns := u16(32), u16(34), u16(36), u16(38)

	tableEnd := 192 + orders + 4*(instruments+samples+patterns)
	if tableEnd > end {
		return bounds, errors.New("truncated MPTM pointer table")
	}

	last := tableEnd

	type section struct {
		start, end int
		label      string
	}

	var sections []section

	span := func(start uint64, size uint64, label string) error {
		if start < uint64(tableEnd) || start > uint64(end) || size > uint64(end)-start {
			return fmt.Errorf("invalid MPTM %s bounds", label)
		}
		last = max(last, int(start+size))
		sections = append(sections, section{int(start), int(start + size), label})
		return nil
	}

	pointer := func(off int) uint64 { return uint64(binary.LittleEndian.Uint32(data[off:])) }
	if data[46]&1 != 0 {
		length := uint64(u16(54))
		if length > 0 {
			if err := span(pointer(56), length, "message"); err != nil {
				return bounds, err
			}
		}
	}

	instrumentTable := 192 + orders
	for i := range instruments {
		ptr := pointer(instrumentTable + i*4)

		if ptr == 0 {
			continue
		}

		if ptr < uint64(tableEnd) || ptr > uint64(end) || 554 > uint64(end)-ptr {
			return bounds, errors.New("invalid MPTM instrument bounds")
		}

		if string(data[int(ptr):int(ptr)+4]) != "IMPI" {
			return bounds, fmt.Errorf("invalid MPTM instrument %d", i+1)
		}

		size := uint64(554)
		marker := string(data[int(ptr)+550 : int(ptr)+554])
		if marker == "XTPM" || marker == "MPTX" {
			size += 120
		}

		if err := span(ptr, size, "instrument"); err != nil {
			return bounds, err
		}
	}

	sampleTable := instrumentTable + instruments*4
	// Non-overlapping compressed streams cannot consume more than the region.
	scanBudget := end - tableEnd
	for i := range samples {
		ptr := pointer(sampleTable + i*4)
		if ptr == 0 {
			continue
		}

		if err := span(ptr, 80, "sample header"); err != nil {
			return bounds, err
		}

		h := data[int(ptr) : int(ptr)+80]
		if string(h[:4]) != "IMPS" {
			return bounds, fmt.Errorf("invalid MPTM sample %d header", i+1)
		}

		u32 := binary.LittleEndian.Uint32

		length, offset := uint64(u32(h[48:])), uint64(u32(h[72:]))
		if h[46] == 0x40 {
			if length != 12 {
				return bounds, fmt.Errorf("invalid MPTM OPL sample %d length", i+1)
			}

			if err := span(offset, 12, "OPL sample"); err != nil {
				return bounds, err
			}
			continue
		}

		allowedConvert := byte(1)
		if h[18]&8 != 0 {
			allowedConvert |= 4
		}

		if h[46]&^allowedConvert != 0 {
			return bounds, fmt.Errorf("unsupported MPTM sample %d encoding %#x", i+1, h[46])
		}

		if length == 0 {
			if offset != 0 {
				if offset < uint64(tableEnd) || offset > uint64(end) {
					return bounds, fmt.Errorf("invalid MPTM empty sample %d offset", i+1)
				}
				last = max(last, int(offset))
			}
			continue
		}

		if offset < uint64(tableEnd) || offset >= uint64(end) {
			return bounds, fmt.Errorf("invalid MPTM sample %d offset", i+1)
		}

		if h[18]&8 != 0 {
			channels := 1
			if h[18]&4 != 0 {
				channels = 2
			}

			at := int(offset)
			for ch := range channels {
				available := min(end-at, scanBudget)
				if length > uint64(available)*8 || length > uint64(^uint(0)>>1) {
					return bounds, fmt.Errorf("invalid MPTM compressed sample %d length", i+1)
				}

				_, used, err := readITCompressed(data[at:at+available], int(length), h[18]&2 != 0, h[46]&4 != 0, false)
				if err != nil {
					return bounds, fmt.Errorf("invalid MPTM compressed sample %d channel %d: %w", i+1, ch+1, err)
				}

				at += used
				scanBudget -= used
			}
			if err := span(offset, uint64(at)-offset, "compressed sample"); err != nil {
				return bounds, err
			}
		} else {
			width, channels := uint64(1), uint64(1)
			if h[18]&2 != 0 {
				width = 2
			}
			if h[18]&4 != 0 {
				channels = 2
			}
			if err := span(offset, length*width*channels, "PCM sample"); err != nil {
				return bounds, err
			}
		}
	}

	patternTable := sampleTable + samples*4
	for i := range patterns {
		ptr := pointer(patternTable + i*4)

		if ptr == 0 {
			continue
		}

		if ptr < uint64(tableEnd) || ptr > uint64(end) || 8 > uint64(end)-ptr {
			return bounds, errors.New("invalid MPTM pattern header bounds")
		}

		length := uint64(binary.LittleEndian.Uint16(data[int(ptr):]))

		if err := span(ptr, 8+length, "pattern"); err != nil {
			return bounds, err
		}
	}

	sort.Slice(sections, func(i, j int) bool { return sections[i].start < sections[j].start })

	for i := 1; i < len(sections); i++ {
		if sections[i].start < sections[i-1].end {
			return bounds, fmt.Errorf("overlapping MPTM %s and %s", sections[i-1].label, sections[i].label)
		}
	}

	if last+4 > end || string(data[last:last+4]) != "XTPM" && string(data[last:last+4]) != "STPM" {
		return bounds, errors.New("MPTM extension header not found at structural boundary")
	}

	return mptExtensionBounds{propertiesStart: last, serializedStart: end, serializedEnd: len(data) - 4}, nil
}
