package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTCarryAfterNoteOffPortamento(t *testing.T) {
	for _, bit122 := range []bool{false, true} {
		for _, bit132 := range []bool{false, true} {
			for _, carry := range []bool{false, true} {
				t.Run(fmt.Sprintf("bit122=%t/bit132=%t/carry=%t", bit122, bit132, carry), func(t *testing.T) {
					fixture := func(carry bool) []byte {
						b := mptSustainPortaFixture(t, bit122, true, false, 2)
						instrument := int(binary.LittleEndian.Uint32(b[198:]))
						flags := byte(1)
						if carry {
							flags |= 8
						}
						setTestEnvelope(b[instrument:instrument+554], 304, flags, []int8{64, 16}, []uint16{0, 12})
						if bit132 {
							bounds, err := locateMPTExtensions(b)
							if err != nil {
								t.Fatal(err)
							}
							field := bytesIndex(t, b, ".FSM")
							length := int(binary.LittleEndian.Uint16(b[field+4:]))
							data := make([]byte, 17)
							copy(data, b[field+6:field+6+length])
							data[mptITCarryAfterNoteOffBit/8] |= 1 << (mptITCarryAfterNoteOffBit % 8)
							out := append([]byte(nil), b[:field]...)
							out = append(out, mptExtensionField(".FSM", uint16(len(data)), data)...)
							out = append(out, b[field+6+length:bounds.serializedStart]...)
							start := len(out)
							out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
							b = binary.LittleEndian.AppendUint32(out, uint32(start))
						}
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(carry), 220)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(false), 220)
					for tick := 7; tick < 10; tick++ {
						ms := tick*20 + 17
						want := channelRMS(ref, ms, ms+2, 0) / channelRMS(baseRef, ms, ms+2, 0)
						got := channelRMS(actual, ms, ms+2, 0) / channelRMS(baseActual, ms, ms+2, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
