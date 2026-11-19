package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMQxxSampleMode(t *testing.T) {
	for _, arg := range []byte{0x02, 0x12, 0x63, 0x72, 0x92, 0xc2, 0xf2} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("Q%02X/compatible=%t", arg, compatible), func(t *testing.T) {
				rows := []byte{0x81, 7, 60, 1, 32, 0, 0x81, 8, 17, arg, 0, 0x81, 8, 17, 0, 0}
				b := mptTwoPCMSamplesFixture(t, rows, true, true)
				b[50] = 4
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				sample := binary.LittleEndian.Uint32(b[194:])
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
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				baseRef, baseActual := rmsWindow(ref, 8, 10), rmsWindow(actual, 8, 10)
				if baseRef < .001 || baseActual < .001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := 4; tick < 12; tick++ {
					want := rmsWindow(ref, tick*20+8, tick*20+10) / baseRef
					got := rmsWindow(actual, tick*20+8, tick*20+10) / baseActual
					if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .10 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
				assertMPTPatternDelayChunking(t, b, referenceRate/5)
			})
		}
	}
}

func TestOpenMPTMQxxInstrumentEnvelope(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, action := range []byte{0, 1, 2, 3} {
			for _, arg := range []byte{0x02, 0x92} {
				t.Run(fmt.Sprintf("compatible=%t/nna=%d/Q%02X", compatible, action, arg), func(t *testing.T) {
					rows := []byte{0x81, 11, 60, 2, 17, arg, 0, 0x81, 8, 17, 0, 0}
					b := mptS70Fixture(t, rows)
					b[50] = 6
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
					mask := byte(1 << (mptITRetrigger % 8))
					b[flag] &^= mask
					if compatible {
						b[flag] |= mask
					}
					instrument := binary.LittleEndian.Uint32(b[198:])
					b[instrument+17] = action
					setTestEnvelope(b[instrument:instrument+554], 304, 1, []int8{64, 32}, []uint16{0, 20})
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					baseRef, baseActual := channelRMS(ref, 19, 20, 0), channelRMS(actual, 19, 20, 0)
					if baseRef < .001 || baseActual < .001 {
						t.Fatal("silent comparison baseline")
					}
					for tick := range 12 {
						ms := tick*20 + 19
						want := channelRMS(ref, ms, ms+1, 0) / baseRef
						got := channelRMS(actual, ms, ms+1, 0) / baseActual
						if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .12 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMQxxOPL(t *testing.T) {
	for _, arg := range []byte{0x02, 0x12, 0x62, 0x72, 0x92, 0xc2, 0xf2} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("Q%02X/compatible=%t", arg, compatible), func(t *testing.T) {
				b := mptS70Fixture(t, []byte{0x81, 7, 60, 2, 24, 0, 0x81, 8, 17, arg, 0, 0x81, 8, 17, 0, 0})
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
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				baseRef, baseActual := rmsWindow(ref, 5, 12), rmsWindow(actual, 5, 12)
				if baseRef < .00001 || baseActual < .00001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := 4; tick < 12; tick++ {
					want := rmsWindow(ref, tick*20+5, tick*20+12) / baseRef
					got := rmsWindow(actual, tick*20+5, tick*20+12) / baseActual
					if math.IsNaN(want) || math.IsNaN(got) || math.Abs(got-want) > .3+.02*want {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}
