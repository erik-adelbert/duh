package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMCombinedPortamentoMemory(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	tests := []struct {
		name string
		rows []byte
	}{
		{"E then L", []byte{0x81, 11, 60, 1, 5, 0x20, 0, 0x81, 9, 64, 12, 0x02, 0, 0x81, 8, 12, 0, 0}},
		{"G then E", []byte{0x81, 11, 60, 1, 7, 0x20, 0, 0x81, 8, 5, 0, 0, 0x81, 8, 5, 0, 0}},
		{"volume pitch then L", []byte{0x81, 7, 60, 1, 110, 0, 0x81, 9, 64, 12, 0x02, 0, 0x81, 8, 12, 0, 0}},
	}
	for _, tt := range tests {
		for _, shared := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/shared=%t/compatible=%t", tt.name, shared, compatible), func(t *testing.T) {
					profile := flags
					if !shared {
						profile &^= 1 << mptITPortaMemoryShare
					}
					b := mptSharedEffectFixture(t, tt.rows, profile)
					if compatible {
						b[44] |= 0x20
					}
					ref, actual := mptSharedEffectRender(t, b)
					for tick := range 6 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .003 {
							t.Errorf("tick %d: frequency OpenMPT %.2f tracker %.2f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
