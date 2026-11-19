package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTNoteCutWithPortamento(t *testing.T) {
	for _, cutStops := range []bool{false, true} {
		for _, porta := range []bool{false, true} {
			t.Run(fmt.Sprintf("stops=%v/porta=%v", cutStops, porta), func(t *testing.T) {
				row := []byte{0x81, 8, 19, 0xc1, 0}
				if porta {
					row = []byte{0x81, 13, 64, 197, 19, 0xc1, 0}
				}

				rows := append([]byte{0x81, 3, 60, 1, 0}, row...)
				rows = append(rows, 0x81, 3, 68, 1, 0)
				b := mptRemainingFlagsFixture(t, rows, false, false, true)
				field := bytesIndex(t, b, ".FSM") + 6 + int(mptITSCxStopsSample)/8
				mask := byte(1 << (mptITSCxStopsSample % 8))
				b[field] &^= mask
				if cutStops {
					b[field] |= mask
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 180)
				if porta {
					assertMPTPCMCorrelation(t, ref, actual, 90, 110)
				} else {
					for _, audio := range [][]float32{ref, actual} {
						if rms := channelRMS(audio, 112, 118, 0); rms > 0.0001 {
							t.Errorf("cut left audible sound: RMS %g", rms)
						}
					}
				}
				assertMPTPCMCorrelation(t, ref, actual, 140, 170)
			})
		}
	}
}

func TestOpenMPTNoteCutFlagChangesPortamento(t *testing.T) {
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 13, 64, 197, 19, 0xc1, 0}
	enabled := mptRemainingFlagsFixture(t, rows, false, false, true)
	disabled := mptRemainingFlagsFixture(t, rows, false, false, false)
	a := referenceMPTMForDuration(t, enabled, 120)
	b := referenceMPTMForDuration(t, disabled, 120)
	if on, off := channelRMS(a, 100, 110, 0), channelRMS(b, 100, 110, 0); on < .001 || off > on*.1 {
		t.Fatalf("reference does not distinguish flag: enabled %g, disabled %g", on, off)
	}
}
