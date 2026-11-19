package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MChannelVolume(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	for _, tt := range []struct{ effect, arg byte }{
		{13, 32}, {13, 0}, {14, 0x08}, {14, 0xf2},
	} {
		t.Run(fmt.Sprintf("%c%02X", 'A'+tt.effect-1, tt.arg), func(t *testing.T) {
			ref, actual := referencePCM(t, referenceS3M(tt.effect, tt.arg, sample))
			refBase, actualBase := referencePCM(t, referenceS3M(0, 0, sample))
			for _, ms := range []int{15, 35, 55, 75, 95, 115} {
				want := rmsWindow(ref, ms, ms+4) / rmsWindow(refBase, ms, ms+4)
				got := rmsWindow(actual, ms, ms+4) / rmsWindow(actualBase, ms, ms+4)
				if math.Abs(got-want) > .04 {
					t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
