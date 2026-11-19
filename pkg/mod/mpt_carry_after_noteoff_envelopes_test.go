package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTCarryAfterNoteOffPitchAndPan(t *testing.T) {
	for _, offset := range []int{386, 468} {
		for _, enabled := range []bool{false, true} {
			for _, carry := range []bool{false, true} {
				t.Run(fmt.Sprintf("offset=%d/enabled=%t/carry=%t", offset, enabled, carry), func(t *testing.T) {
					b := mptCarryAfterNoteOffFixture(t, enabled, false, 255)
					instrument := int(binary.LittleEndian.Uint32(b[198:]))
					flags := byte(1)
					if carry {
						flags |= 8
					}
					setTestEnvelope(b[instrument:instrument+554], offset, flags, []int8{-16, 16}, []uint16{0, 12})
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					if offset == 468 {
						assertMPTPCMCorrelation(t, ref, actual, 155, 210)
						return
					}
					for _, ms := range []int{155, 175, 195} {
						l, r := channelRMS(ref, ms, ms+5, 0), channelRMS(ref, ms, ms+5, 1)
						al, ar := channelRMS(actual, ms, ms+5, 0), channelRMS(actual, ms, ms+5, 1)
						want, got := l/(l+r), al/(al+ar)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
							t.Errorf("%dms: OpenMPT pan %g, tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}
