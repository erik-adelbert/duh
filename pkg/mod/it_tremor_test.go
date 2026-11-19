package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITTremor(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"I34 and memory", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0x81, 8, 9, 0, 0}},
		{"I11", []byte{0x81, 11, 60, 1, 9, 0x11, 0, 0x81, 8, 9, 0, 0}},
		{"I03", []byte{0x81, 11, 60, 1, 9, 0x03, 0, 0x81, 8, 9, 0, 0}},
		{"I30", []byte{0x81, 11, 60, 1, 9, 0x30, 0, 0x81, 8, 9, 0, 0}},
		{"interrupted", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0, 0x81, 8, 9, 0, 0}},
		{"new note", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0x81, 11, 60, 1, 9, 0, 0}},
		{"before note", []byte{0x81, 8, 9, 0x34, 0, 0x81, 11, 60, 1, 9, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/old=%t", tt.name, legacy, old), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 6, true, legacy)
					if old {
						b[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 360, 256)
					for tick := range 18 {
						ms := tick*20 + 15
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.IsNaN(got) || (want > .001) != (got > .001) {
							t.Errorf("tick %d: OpenMPT RMS %.4f tracker %.4f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
