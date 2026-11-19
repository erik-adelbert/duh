package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTS3MTremor(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 224
	}
	for _, arg := range []byte{0x11, 0x23, 0x01, 0x10} {
		t.Run(fmt.Sprintf("I%02X", arg), func(t *testing.T) {
			ref, actual := referencePCM(t, referenceS3M(9, arg, sample))
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
