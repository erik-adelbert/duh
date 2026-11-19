package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMEmbeddedFilterMacrosBeforeNote(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, fixed := range []bool{false, true} {
		for _, resonance := range []bool{false, true} {
			t.Run(fmt.Sprintf("fixed=%t/resonance=%t", fixed, resonance), func(t *testing.T) {
				fixture := func(value byte) []byte {
					parametered := map[int]string{0: "F0F000z"}
					fixedMacros := map[int]string{}
					var rows []byte
					if resonance {
						rows = append(rows, 0x81, 8, 26, 64, 0)
					}
					arg := value
					if fixed {
						code := "00"
						if resonance {
							code = "01"
						}
						fixedMacros[0] = fmt.Sprintf("F0F0%s%02X", code, value)
						arg = 0x80
					} else if resonance {
						parametered[1] = "F0F001z"
						rows = append(rows, 0x81, 8, 19, 0xf1, 0)
					}
					rows = append(rows, 0x81, 8, 26, arg, 0, 0x81, 3, 60, 1, 0, 0, 0)
					return embeddedMacroFixture(t, mptSharedEffectFixture(t, rows, flags), -1, parametered, fixedMacros)
				}
				value, baseline := byte(32), byte(96)
				if resonance {
					value, baseline = 96, 0
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(value), 240)
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(baseline), 240)
				wantBase, gotBase := rmsWindow(baseRef, 180, 220), rmsWindow(baseActual, 180, 220)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatal("silent comparison baseline")
				}
				want, got := rmsWindow(ref, 180, 220)/wantBase, rmsWindow(actual, 180, 220)/gotBase
				if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) ||
					math.Abs(want-got) > .09 || math.Abs(want-1) < .05 {
					t.Fatalf("relative RMS: OpenMPT %.4f, tracker %.4f", want, got)
				}
			})
		}
	}
}
