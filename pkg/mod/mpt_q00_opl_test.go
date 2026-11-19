package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQ00OPLNote(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, memory := range []byte{0, 0x02, 0x92} {
			t.Run(fmt.Sprintf("compatible=%t/memory=%02X", compatible, memory), func(t *testing.T) {
				rows := []byte{0x81, 7, 60, 2, 24, 0}
				if memory == 0 {
					rows = append(rows, 0)
				} else {
					rows = append(rows, 0x81, 8, 17, memory, 0)
				}
				rows = append(rows, 0x81, 15, 60, 2, 24, 17, 0, 0, 0x81, 8, 17, 0, 0)
				b := mptS70Fixture(t, rows)
				b[50] = 4
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				instrument := binary.LittleEndian.Uint32(b[198:])
				for note := range 120 {
					b[instrument+65+uint32(note)*2] = 2
				}
				sample := binary.LittleEndian.Uint32(b[206:])
				patch := binary.LittleEndian.Uint32(b[sample+72:])
				copy(b[patch:patch+12], percussiveOPLPatch[:])
				ref, actual := mptSharedEffectRenderForDuration(t, b, 320)
				baseRef, baseActual := rmsWindow(ref, 5, 12), rmsWindow(actual, 5, 12)
				if baseRef < .00001 || baseActual < .00001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := 8; tick < 16; tick++ {
					want := rmsWindow(ref, tick*20+5, tick*20+12) / baseRef
					got := rmsWindow(actual, tick*20+5, tick*20+12) / baseActual
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .3+.02*want {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
				assertMPTPatternDelayChunking(t, b, 14000)
			})
		}
	}
}
