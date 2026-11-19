package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MVibratoSampleRates(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, rate := range []uint32{4182, 16726} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			data := referenceS3M(21, 0x4f, sample)
			binary.LittleEndian.PutUint32(data[0x100+0x20:], rate)
			ref, actual := referencePCM(t, data)
			for tick := 0; tick < 6; tick++ {
				want := referenceTickFrequency(t, ref, tick)
				got := referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .005 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
