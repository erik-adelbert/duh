package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTSustainPortamentoFollowup(t *testing.T) {
	tests := []struct {
		name string
		tail []byte
	}{
		{"second note-off", []byte{0x81, 1, 255, 0}},
		{"second porta change", []byte{0x81, 11, 67, 2, 7, 4, 0}},
		{"new note", []byte{0x81, 3, 60, 2, 0}},
		{"retrigger", []byte{0x81, 8, 17, 1, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/compatible=%t", tt.name, enabled, compatible), func(t *testing.T) {
					b := mptSustainPortaFixtureWithTail(t, enabled, compatible, true, 1, true, tt.tail)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
					assertMPTPCMCorrelation(t, ref, actual, 200, 240)
				})
			}
		}
	}
}
