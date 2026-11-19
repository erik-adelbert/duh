package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMInstrumentAfterSpecialNote(t *testing.T) {
	for _, special := range []byte{253, 254, 255} {
		for _, enabled := range []bool{false, true} {
			for _, emptyMap := range []bool{false, true} {
				t.Run(fmt.Sprintf("note=%d/flag=%t/emptyMap=%t", special, enabled, emptyMap), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, special, 0, 0x81, 2, 1, 0}
					b := mptInstrumentWithoutNoteFixture(t, rows, enabled, 0)
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITEmptyNoteMapSlot)/8
					mask := byte(1 << (mptITEmptyNoteMapSlot % 8))
					b[flag] &^= mask
					if emptyMap {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
					for ms := 20; ms < 290; ms += 10 {
						if ms%60 == 0 || ms == 130 {
							continue
						}
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(got-want) > .003 {
							t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}
