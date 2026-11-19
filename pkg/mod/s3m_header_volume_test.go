package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MHeaderGlobalVolume(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 192
	}
	for _, volume := range []byte{0, 32, 64} {
		t.Run(fmt.Sprint(volume), func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x30] = volume
			ref, actual := referencePCM(t, data)
			refBase, actualBase := referencePCM(t, referenceS3M(0, 0, sample))
			want := rmsWindow(ref, 15, 19) / rmsWindow(refBase, 15, 19)
			got := rmsWindow(actual, 15, 19) / rmsWindow(actualBase, 15, 19)
			if math.Abs(got-want) > .02 {
				t.Errorf("OpenMPT ratio %.3f tracker %.3f", want, got)
			}
		})
	}
}
