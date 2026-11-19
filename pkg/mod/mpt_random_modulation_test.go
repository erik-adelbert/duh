package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMRandomVibratoTremolo(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, family := range []byte{0x30, 0x40} {
		for _, old := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/old=%t", family|3, old), func(t *testing.T) {
				effect := byte(8)
				if family == 0x40 {
					effect = 18
				}
				rows := []byte{0x81, 15, 60, 1, 32, 19, family | 3, 0}
				for range 8 {
					rows = append(rows, 0x81, 8, effect, 0x4f, 0)
				}
				fixture := func(rows []byte) []byte {
					b := mptSharedEffectFixture(t, rows, flags)
					b[50] = 3
					if old {
						b[44] |= 0x10
					}
					return b
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(rows), 540)
				refBase, actualBase := mptSharedEffectRenderForDuration(t, fixture([]byte{0x81, 7, 60, 1, 32, 0}), 540)
				for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
					base := refBase
					if name == "tracker" {
						base = actualBase
					}
					minimum, maximum := math.Inf(1), math.Inf(-1)
					for tick := 3; tick < 27; tick++ {
						var value float64
						if family == 0x30 {
							value = referenceTickFrequency(t, pcm, tick)/referenceTickFrequency(t, base, tick) - 1
						} else {
							ms := tick*20 + 19
							value = channelRMS(pcm, ms, ms+1, 0)/channelRMS(base, ms, ms+1, 0) - 1
						}
						if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1 {
							t.Fatalf("%s tick %d: invalid modulation %v", name, tick, value)
						}
						minimum, maximum = min(minimum, value), max(maximum, value)
					}
					if maximum-minimum < .01 {
						t.Errorf("%s: modulation did not vary: min=%v max=%v", name, minimum, maximum)
					}
				}
			})
		}
	}
}
