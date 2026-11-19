package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMTremorTremoloOPL(t *testing.T) {
	tests := []struct {
		name   string
		rows   []byte
		tremor bool
	}{
		{"tremor", []byte{0x81, 11, 60, 2, 9, 0x34, 0, 0x81, 8, 9, 0, 0}, true},
		{"tremolo", []byte{0x81, 15, 60, 2, 32, 18, 0x4f, 0, 0x81, 8, 18, 0, 0}, false},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/compatible=%t", tt.name, compatible), func(t *testing.T) {
				flags := all
				if !compatible {
					flags &^= 1<<mptITTremor | 1<<mptITVibratoTremoloPanbrello
				}
				fixture := func(rows []byte) []byte {
					b := mptSharedEffectFixture(t, rows, flags)
					b[50] = 6
					return b
				}
				ref, actual := mptSharedEffectRenderForDuration(t, fixture(tt.rows), 240)
				baseRows := []byte{0x81, 7, 60, 2, 64, 0}
				if !tt.tremor {
					baseRows[4] = 32
				}
				baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(baseRows), 240)
				for tick := range 12 {
					ms := tick*20 + 15
					want := rmsWindow(ref, ms, ms+5) / rmsWindow(baseRef, ms, ms+5)
					got := rmsWindow(actual, ms, ms+5) / rmsWindow(baseActual, ms, ms+5)
					if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) {
						t.Fatalf("tick %d: invalid ratio OpenMPT %g tracker %g", tick, want, got)
					}
					if tt.tremor && (want > .1) != (got > .1) {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
					if !tt.tremor && math.Abs(got-want) > .3 {
						t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}
