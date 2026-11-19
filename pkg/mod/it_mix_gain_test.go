package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestITHeaderMixGain(t *testing.T) {
	for _, header := range []byte{0, 16, 32, 48, 64, 88, 128, 200} {
		t.Run(fmt.Sprint(header), func(t *testing.T) {
			b := testIT()
			b[49] = header
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			want := float32(min(header, 128)) / 128
			if got := loaded.Assets.ITMixGain; got != want {
				t.Fatalf("mix gain = %g, want %g", got, want)
			}
			ref, actual := referencePCM(t, b)
			refLevel, actualLevel := rmsWindow(ref, 10, 30), rmsWindow(actual, 10, 30)
			if math.Abs(actualLevel-refLevel) > 0.001 {
				t.Fatalf("tracker RMS %g, OpenMPT RMS %g", actualLevel, refLevel)
			}
		})
	}
}
