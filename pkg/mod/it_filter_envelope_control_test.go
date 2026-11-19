package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITS7FilterEnvelopeOpenMPT(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/enabled=%t", legacy, enabled), func(t *testing.T) {
				fixture := func(control bool) []byte {
					b := testITInstrument()
					b[50] = 2
					h := b[len(b)-554:]
					h[304], h[58], h[59] = 0, 128|80, 128|40
					flags := byte(0x80)
					if enabled {
						flags |= 1
					}
					setTestEnvelope(h, 468, flags, []int8{-24, 16}, []uint16{0, 12})
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					rows := []byte{0x81, 3, 60, 1, 0}
					if control {
						rows = append(rows, 0x81, 8, 19, 0x7c, 0, 0x81, 8, 19, 0x7b, 0)
					}
					packed := append(rows, make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					return b
				}
				ref, actual := referencePCMWithChunks(t, fixture(true), "2", 120, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture(false), "2", 120, 256)
				for _, ms := range []int{35, 55, 75, 95, 115} {
					want := channelRMS(ref, ms, ms+4, 0) / channelRMS(refBase, ms, ms+4, 0)
					got := channelRMS(actual, ms, ms+4, 0) / channelRMS(actualBase, ms, ms+4, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .18 {
						t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}
