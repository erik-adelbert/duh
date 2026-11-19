package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITS7PastNotesOpenMPT(t *testing.T) {
	for _, arg := range []byte{0x70, 0x71, 0x72} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/legacy=%t", arg, legacy), func(t *testing.T) {
				fixture := func(control byte) []byte {
					b := testITInstrument()
					b[50] = 2
					h := b[len(b)-554:]
					h[17], h[304] = 1, 0
					binary.LittleEndian.PutUint16(h[20:], 128)
					for i := range 4096 {
						binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					rows := []byte{0x81, 3, 60, 1, 0, 0x81, 1, 60, 0, 0x81, 8, 19, control, 0}
					packed := append(rows, make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					return b
				}
				ref, actual := referencePCMWithChunks(t, fixture(arg), "1", 120, 256)
				refBase, actualBase := referencePCMWithChunks(t, fixture(0x70), "1", 120, 256)
				for _, ms := range []int{99, 119} {
					want := channelRMS(ref, ms-1, ms, 0) / channelRMS(refBase, ms-1, ms, 0)
					got := channelRMS(actual, ms-1, ms, 0) / channelRMS(actualBase, ms-1, ms, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .12 {
						t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}
