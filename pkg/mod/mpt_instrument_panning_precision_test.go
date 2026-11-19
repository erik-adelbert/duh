package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMExtendedPanningPrecision(t *testing.T) {
	for _, pan := range []uint32{1, 103, 127, 129, 193, 255} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("pan=%d/enabled=%t", pan, enabled), func(t *testing.T) {
				b := mptInstrumentPanFixture(t, pan, enabled)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
				leftRef, rightRef := channelRMS(ref, 30, 70, 0), channelRMS(ref, 30, 70, 1)
				left, right := channelRMS(actual, 30, 70, 0), channelRMS(actual, 30, 70, 1)
				if leftRef+rightRef < .001 || left+right < .001 ||
					math.Abs(right/(left+right)-rightRef/(leftRef+rightRef)) > .002 {
					t.Fatalf("OpenMPT left/right %g/%g, tracker %g/%g", leftRef, rightRef, left, right)
				}
				assertMPTPatternDelayChunking(t, b, 4800)
			})
		}
	}
}
