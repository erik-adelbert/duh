package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITTremolo(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"memory", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0x81, 8, 18, 0, 0}},
		{"partial memory", []byte{0x81, 15, 60, 1, 32, 18, 0x45, 0, 0x81, 8, 18, 0x80, 0, 0x81, 8, 18, 0x03, 0}},
		{"interrupt", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0, 0x81, 8, 18, 0, 0}},
		{"new note", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0x81, 15, 60, 1, 32, 18, 0, 0}},
		{"zero volume", []byte{0x81, 15, 60, 1, 0, 18, 0x4f, 0, 0x81, 8, 18, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/old=%t", tt.name, legacy, old), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 6, true, legacy)
					baseline := testITPitchFixture([]byte{0x81, 7, 60, 1, 32, 0}, 6, true, legacy)
					if old {
						b[44] |= 0x10
						baseline[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 360, 256)
					refBase, actualBase := referencePCMWithChunks(t, baseline, "2", 360, 256)
					for tick := range 18 {
						ms := tick*20 + 19
						want := channelRMS(ref, ms, ms+1, 0) / channelRMS(refBase, ms, ms+1, 0)
						got := channelRMS(actual, ms, ms+1, 0) / channelRMS(actualBase, ms, ms+1, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .04 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
