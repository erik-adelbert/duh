package mod

import (
	"math"
	"testing"
)

func TestOpenMPTITDelayedVolumeSlide(t *testing.T) {
	for _, tt := range []struct {
		name         string
		command      byte
		sampleVolume byte
	}{
		{"down", 104, 64},
		{"up", 94, 32},
	} {
		for _, legacy := range []bool{false, true} {
			name := tt.name + "/IT"
			if legacy {
				name = tt.name + "/ModPlug"
			}
			t.Run(name, func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 15, 60, 1, tt.command, 19, 0xd1, 0}
				b := testITPitchFixture(rows, 3, true, legacy)
				b[256+19] = tt.sampleVolume
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase := channelRMS(ref, 46, 49, 0)
				actualBase := channelRMS(actual, 46, 49, 0)
				if refBase < 1e-6 || actualBase < 1e-6 {
					t.Fatal("silent comparison baseline")
				}
				for _, ms := range []int{66, 116} {
					want := channelRMS(ref, ms, ms+3, 0) / refBase
					got := channelRMS(actual, ms, ms+3, 0) / actualBase
					if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .035 {
						t.Errorf("%dms: relative volume OpenMPT %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}
