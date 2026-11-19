package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MMasterVolume(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 160
	}

	for _, master := range []byte{0, 2, 16, 18, 32, 48, 64, 0x80, 0x90, 0xb0, 0xc0} {
		t.Run(fmt.Sprintf("%02X", master), func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x33] = master
			ref, actual := referencePCM(t, data)
			want, got := rmsWindow(ref, 15, 19), rmsWindow(actual, 15, 19)
			if math.Abs(got/want-1) > .03 {
				t.Errorf("OpenMPT RMS %.6f, tracker %.6f", want, got)
			}
		})
	}
}
