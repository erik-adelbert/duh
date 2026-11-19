package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITInstrumentOnly(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"same instrument", []byte{0x81, 3, 60, 1, 0, 0x81, 2, 1, 0}},
		{"new instrument", []byte{0x81, 3, 60, 1, 0, 0x81, 2, 2, 0}},
		{"instrument before note", []byte{0x81, 2, 2, 0, 0x81, 1, 60, 0}},
		{"after portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 9, 72, 7, 0, 0, 0x81, 2, 1, 0}},
		{"volume column", []byte{0x81, 3, 60, 1, 0, 0x81, 6, 2, 40, 0}},
		{"after note cut", []byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0x81, 2, 2, 0}},
		{"after note off", []byte{0x81, 3, 60, 1, 0, 0x81, 1, 255, 0, 0x81, 2, 2, 0}},
		{"effect volume slide", []byte{0x81, 3, 60, 1, 0, 0x81, 10, 2, 4, 2, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				fixture := func(rows []byte) []byte {
					b, assets := sameFormatInstrumentFixture(false)
					b = sameFormatInstrumentMode(b, &assets)
					b[50] = 2
					binary.LittleEndian.PutUint16(b[40:], 0x214)
					binary.LittleEndian.PutUint16(b[42:], 0x214)
					for _, header := range []int{256, 8704} {
						offset := int(binary.LittleEndian.Uint32(b[header+72:]))
						for i := range 4096 {
							binary.LittleEndian.PutUint16(b[offset+i*2:], 12000)
						}
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					first := int(binary.LittleEndian.Uint32(b[194:]))
					b[first+65+72*2] = 2
					pattern := int(binary.LittleEndian.Uint32(b[210:]))
					packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
					copy(b[pattern+8:], packed)
					return b
				}
				ref, actual := referencePCMWithChunks(t, fixture(tt.rows), "2", 140, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture([]byte{0x81, 3, 60, 1, 0}), "2", 140, 256)
				for _, ms := range []int{38, 78, 118, 138} {
					want := channelRMS(ref, ms-1, ms, 0) / channelRMS(refBase, ms-1, ms, 0)
					got := channelRMS(actual, ms-1, ms, 0) / channelRMS(actualBase, ms-1, ms, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
						t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}
