package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITFineTune(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"with note", []byte{0x81, 11, 60, 1, 19, 0x25, 0}},
		{"after note", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0x2f, 0}},
		{"after slide", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 6, 8, 0, 0x81, 8, 19, 0x20, 0}},
		{"memory", []byte{0x81, 11, 60, 1, 19, 0x25, 0, 0x81, 8, 19, 0, 0}},
		{"next note", []byte{0x81, 11, 60, 1, 19, 0x25, 0, 0x81, 3, 72, 1, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/linear=%t", tt.name, legacy, linear), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 2, linear, legacy)
					ref, actual := referencePCMWithChunks(t, b, "2", 120, 256)
					base := testITPitchFixture([]byte{0x81, 3, 60, 1, 0}, 2, linear, legacy)
					refBaseline, actualBaseline := referencePCMWithChunks(t, base, "2", 120, 256)
					refBase, actualBase := referenceTickFrequency(t, refBaseline, 0), referenceTickFrequency(t, actualBaseline, 0)
					for tick := range 6 {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.Abs(got-want) > .005 {
							t.Errorf("tick %d: OpenMPT %.5f tracker %.5f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
