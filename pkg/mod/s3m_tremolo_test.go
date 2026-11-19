package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MTremolo(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	for _, arg := range []byte{0x24, 0x47} {
		t.Run(fmt.Sprintf("R%02X", arg), func(t *testing.T) {
			data := referenceS3M(18, arg, sample)
			data[0x100+0x1c] = 32
			baseline := referenceS3M(0, 0, sample)
			baseline[0x100+0x1c] = 32
			ref, actual := referencePCM(t, data)
			refBase, actualBase := referencePCM(t, baseline)
			for _, ms := range []int{15, 35, 55, 75, 95, 110} {
				want := rmsWindow(ref, ms, ms+4) / rmsWindow(refBase, ms, ms+4)
				got := rmsWindow(actual, ms, ms+4) / rmsWindow(actualBase, ms, ms+4)
				if math.Abs(got-want) > .06 {
					t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
