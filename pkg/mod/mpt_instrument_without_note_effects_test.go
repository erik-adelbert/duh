package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMInstrumentWithoutNoteEffects(t *testing.T) {
	tests := []struct {
		name string
		cell []byte
	}{
		{"portamento", []byte{0x81, 10, 1, 7, 4, 0}},
		{"volume-portamento", []byte{0x81, 6, 1, 193, 0}},
		{"offset", []byte{0x81, 10, 1, 15, 2, 0}},
		{"volume", []byte{0x81, 6, 1, 32, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/flag=%t", tt.name, enabled), func(t *testing.T) {
				rows := append([]byte{0x81, 3, 60, 2, 0}, tt.cell...)
				b := mptInstrumentWithoutNoteFixture(t, rows, enabled, 0)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
				for ms := 20; ms < 290; ms += 10 {
					if ms == 60 {
						continue
					}
					want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
					if math.Abs(got-want) > .003 {
						t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
					}
				}
			})
		}
	}
}
