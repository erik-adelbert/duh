package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMSpeedOnePortamento(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	tests := []struct {
		name string
		rows []byte
	}{
		{"effect column", []byte{0x81, 11, 60, 1, 7, 0x10, 0,
			0x81, 9, 64, 12, 0x02, 0, 0x81, 8, 12, 0, 0,
			0x81, 8, 5, 0x10, 0, 0x81, 8, 5, 0, 0}},
		{"fine and extra fine", []byte{0x81, 11, 60, 1, 5, 0xf2, 0,
			0x81, 8, 6, 0xe2, 0, 0x81, 8, 5, 0xf2, 0}},
		{"volume pitch", []byte{0x81, 7, 60, 1, 120, 0,
			0x81, 4, 120, 0, 0x81, 4, 120, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%t", tt.name, enabled), func(t *testing.T) {
				profile := flags
				if !enabled {
					profile &^= 1 << mptSlidesAtSpeed1
				}
				b := mptSharedEffectFixture(t, tt.rows, profile)
				b[50] = 1
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
