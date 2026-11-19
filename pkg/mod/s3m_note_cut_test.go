package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTS3MNoteCut(t *testing.T) {
	for _, arg := range []byte{0xc0, 0xc1, 0xc3} {
		t.Run(fmt.Sprintf("S%02X", arg), func(t *testing.T) {
			sample := make([]byte, 8192)
			for i := range sample {
				sample[i] = 224
			}
			data := referenceS3M(19, arg, sample)
			ref, actual := referencePCM(t, data)
			for _, ms := range []int{5, 25, 45, 65, 85, 105} {
				want := rmsWindow(ref, ms, ms+5) > .001
				got := rmsWindow(actual, ms, ms+5) > .001
				if got != want {
					t.Errorf("%dms: OpenMPT audible=%t tracker audible=%t", ms, want, got)
				}
			}
		})
	}
}
