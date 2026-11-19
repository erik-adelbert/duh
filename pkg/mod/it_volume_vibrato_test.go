package mod

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestITVolume203OpenMPT(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"vibrato memory", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 4, 203, 0}},
		{"no vibrato memory", []byte{0x81, 7, 60, 1, 203, 0, 0x81, 4, 203, 0}},
		{"new note", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 7, 60, 1, 203, 0}},
		{"after volume slide", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 4, 88, 0, 0x81, 4, 203, 0}},
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
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.IsNaN(got) || math.Abs(got/want-1) > .0015 {
							t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITVolume203Validation(t *testing.T) {
	if _, err := unpackITPattern([]byte{0x81, 4, 203, 0}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackITPattern([]byte{0x81, 12, 203, 8, 0x82, 0}, 1); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
		t.Fatalf("expected combination rejection, got %v", err)
	}
}

func TestOpenMPTITVolumeVibratoFamily(t *testing.T) {
	for volume := byte(203); volume <= 212; volume++ {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("volume=%d/legacy=%t/old=%t", volume, legacy, old), func(t *testing.T) {
					rows := []byte{0x81, 11, 60, 1, 8, 0x82, 0,
						0x81, 4, volume, 0, 0x81, 4, 203, 0}
					b := testITPitchFixture(rows, 6, true, legacy)
					if old {
						b[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 360, 256)
					for tick := range 18 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.IsNaN(got) || math.Abs(got/want-1) > .0015 {
							t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}
