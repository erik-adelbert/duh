package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MFineTune(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, arg := range []byte{0x20, 0x27, 0x28, 0x2f} {
		t.Run(fmt.Sprintf("S%02X", arg), func(t *testing.T) {
			ref, actual := referencePCM(t, referenceS3M(19, arg, sample))
			want := referenceTickFrequency(t, ref, 1)
			got := referenceTickFrequency(t, actual, 1)
			if math.Abs(got/want-1) > .003 {
				t.Errorf("OpenMPT %.2f Hz tracker %.2f Hz", want, got)
			}
		})
	}
}
