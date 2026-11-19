package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMRandomPanbrello(t *testing.T) {
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, sampleAndHold := range []bool{false, true} {
		for _, hold := range []bool{false, true} {
			t.Run(fmt.Sprintf("sampleAndHold=%t/hold=%t", sampleAndHold, hold), func(t *testing.T) {
				flags := all &^ (1<<mptITSampleAndHoldPanbrello | 1<<mptITPanbrelloHold)
				if sampleAndHold {
					flags |= 1 << mptITSampleAndHoldPanbrello
				}
				if hold {
					flags |= 1 << mptITPanbrelloHold
				}
				rows := []byte{0x81, 15, 60, 1, 32, 19, 0x53, 0}
				for range 8 {
					rows = append(rows, 0x81, 8, 25, 0x4f, 0)
				}
				b := mptSharedEffectFixture(t, rows, flags)
				b[50] = 3
				ref, actual := mptSharedEffectRenderForDuration(t, b, 540)
				for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
					var previous float64
					changes, held := 0, 0
					for tick := 3; tick < 27; tick++ {
						ms := tick*20 + 19
						l, r := channelRMS(pcm, ms, ms+1, 0), channelRMS(pcm, ms, ms+1, 1)
						value := (r - l) / (r + l)
						if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1 {
							t.Fatalf("%s tick %d: invalid balance %v", name, tick, value)
						}
						if tick > 3 && math.Abs(value-previous) > .08 {
							changes++
						}
						if tick > 3 && math.Abs(value-previous) <= .08 {
							held++
						}
						if sampleAndHold && (tick-3)%4 != 0 && math.Abs(value-previous) > .08 {
							t.Errorf("%s tick %d: sample-and-hold changed from %.3f to %.3f", name, tick, previous, value)
						}
						previous = value
					}
					if changes == 0 || sampleAndHold && held < 12 || !sampleAndHold && changes < 3 {
						t.Errorf("%s: changes=%d held=%d", name, changes, held)
					}
				}
			})
		}
	}
}
