package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MVibrato(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}

	for _, tt := range []struct {
		effect byte
		arg    byte
	}{
		{8, 0x4f}, {8, 0x23}, {21, 0x4f}, {21, 0x23},
	} {
		t.Run(fmt.Sprintf("%c%02X", 'A'+tt.effect-1, tt.arg), func(t *testing.T) {
			ref, actual := referencePCM(t, referenceS3M(tt.effect, tt.arg, sample))
			for tick := 0; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick)
				got := referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .004 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
