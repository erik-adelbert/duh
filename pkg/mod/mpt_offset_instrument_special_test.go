package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTOffsetWithInstrumentAfterSpecialNote(t *testing.T) {
	for _, special := range []byte{253, 254, 255} {
		for _, clearMemory := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("special=%d/clear=%t/enabled=%t", special, clearMemory, enabled), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, special, 0, 0x81, 10, 2, 15, 1, 0}
					b := mptOffsetInstrumentFixture(t, rows, enabled, true)
					instrument := int(binary.LittleEndian.Uint32(b[198:]))
					setTestEnvelope(b[instrument:instrument+554], 304, 1, []int8{64}, []uint16{0})
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITClearOldNoteAfterCut)/8
					if !clearMemory {
						b[flag] &^= 1 << (mptITClearOldNoteAfterCut % 8)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
					r, a := rmsWindow(ref, 140, 180), rmsWindow(actual, 140, 180)
					if math.Abs(r-a) > .005 {
						t.Fatalf("reference RMS %g, actual RMS %g", r, a)
					}
					if r > .001 {
						assertMPTPCMCorrelation(t, ref, actual, 140, 180)
					}
				})
			}
		}
	}
}
