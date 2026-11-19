package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MPanningSlide(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	for _, arg := range []byte{0x04, 0x40, 0xf2, 0x2f} {
		t.Run(fmt.Sprintf("P%02X", arg), func(t *testing.T) {
			data := referenceS3M(16, arg, sample)
			data[0x33] = 0x30
			ref, actual := referencePCM(t, data)
			for _, ms := range []int{15, 35, 55, 75, 95, 115} {
				ratio := func(pcm []float32) float64 {
					var left, right float64
					for i := ms * referenceRate / 1000; i < (ms+4)*referenceRate/1000; i++ {
						left += math.Abs(float64(pcm[2*i]))
						right += math.Abs(float64(pcm[2*i+1]))
					}
					return right / (left + right)
				}
				want, got := ratio(ref), ratio(actual)
				if math.IsNaN(got) || math.Abs(got-want) > .02 {
					t.Errorf("%dms: OpenMPT pan %.3f tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
