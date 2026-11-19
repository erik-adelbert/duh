package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITVolumePitchSlideFamily(t *testing.T) {
	for volume := byte(105); volume <= 124; volume++ {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("volume=%d/legacy=%t/linear=%t", volume, legacy, linear), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 1, 0, 0x81, 4, volume, 0, 0x81, 4, volume, 0}
					b := testITPitchFixture(rows, 2, linear, legacy)
					ref, actual := referencePCMWithChunks(t, b, "2", 120, 256)
					refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
					for tick := 1; tick < 6; tick++ {
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
