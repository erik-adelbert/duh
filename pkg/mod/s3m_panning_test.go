package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MSpecialPanning(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 224
	}
	for _, arg := range []byte{0x80, 0x88, 0x8f, 0x91} {
		t.Run(fmt.Sprintf("S%02X", arg), func(t *testing.T) {
			ref, actual := referencePCM(t, referenceS3M(19, arg, sample))
			levels := func(pcm []float32) (float64, float64) {
				var left, right float64
				for i := 480; i < 960; i++ {
					left += float64(pcm[2*i])
					right += float64(pcm[2*i+1])
				}
				return left, right
			}
			rl, rr := levels(ref)
			al, ar := levels(actual)
			want := rr / (math.Abs(rl) + math.Abs(rr))
			got := ar / (math.Abs(al) + math.Abs(ar))
			if math.IsNaN(got) || math.Abs(got-want) > .04 {
				t.Errorf("normalized right: OpenMPT %.3f tracker %.3f", want, got)
			}
		})
	}
}
