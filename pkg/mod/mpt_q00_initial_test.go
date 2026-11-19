package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQ00InitialNote(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, speed := range []byte{1, 2, 4, 6} {
			t.Run(fmt.Sprintf("compatible=%t/speed=%d", compatible, speed), func(t *testing.T) {
				b := mptS70Fixture(t, []byte{0x81, 15, 60, 2, 32, 17, 0, 0, 0x81, 8, 17, 0, 0})
				b[50] = speed
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				sample := binary.LittleEndian.Uint32(b[202:])
				b[sample+18] |= 0x10
				binary.LittleEndian.PutUint32(b[sample+52:], 128)
				binary.LittleEndian.PutUint32(b[sample+56:], 2048)
				binary.LittleEndian.PutUint32(b[sample+60:], 48000)
				data := binary.LittleEndian.Uint32(b[sample+72:])
				for i := range 4096 {
					var value uint16
					if i < 700 {
						value = 12000
					}
					binary.LittleEndian.PutUint16(b[data+uint32(i)*2:], value)
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, int(speed)*40+20)
				baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				if baseRef < .001 || baseActual < .001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := 0; tick < int(speed)*2; tick++ {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
					got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .10 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
				assertMPTPatternDelayChunking(t, b, int(speed)*2*960)
			})
		}
	}
}
