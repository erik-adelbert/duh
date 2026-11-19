package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQ63(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		t.Run(fmt.Sprintf("compatible=%t", compatible), func(t *testing.T) {
			rows := []byte{0x81, 15, 60, 2, 59, 17, 0x63, 0, 0x81, 8, 17, 0, 0}
			b := mptS70Fixture(t, rows)
			flags := bytesIndex(t, b, ".FSM") + 6
			if !compatible {
				b[flags+2] &^= 0x80
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 75)
			for tick := range 4 {
				r := rmsWindow(ref, tick*20+5, tick*20+10)
				a := rmsWindow(actual, tick*20+5, tick*20+10)
				if r < .001 || math.Abs(r-a) > .05*r {
					t.Errorf("tick %d: OpenMPT %g tracker %g", tick, r, a)
				}
			}
		})
	}
}

func TestOpenMPTMQ63SampleRestart(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		t.Run(fmt.Sprintf("compatible=%t", compatible), func(t *testing.T) {
			rows := []byte{0x81, 15, 60, 2, 59, 17, 0x63, 0, 0x81, 8, 17, 0, 0}
			b := mptS70Fixture(t, rows)
			b[50] = 6
			flags := bytesIndex(t, b, ".FSM") + 6
			if !compatible {
				b[flags+2] &^= 0x80
			}
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			b[sample+18] &^= 0x10
			binary.LittleEndian.PutUint32(b[sample+60:], 48000)
			pcm := int(binary.LittleEndian.Uint32(b[sample+72:]))
			for i := range 4096 {
				var value uint16
				if i < 700 {
					value = 16000
				}
				binary.LittleEndian.PutUint16(b[pcm+i*2:], value)
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
			for tick := range 12 {
				r, a := rmsWindow(ref, tick*20+8, tick*20+10), rmsWindow(actual, tick*20+8, tick*20+10)
				if (tick == 0 || tick == 3 || tick == 6) && r < .001 {
					t.Errorf("reference did not restart on tick %d", tick)
				}
				if math.Abs(r-a) > .015 {
					t.Errorf("tick %d: OpenMPT %g tracker %g", tick, r, a)
				}
			}
		})
	}
}
