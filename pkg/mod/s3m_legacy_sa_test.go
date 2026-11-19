package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MLegacyStereoControl(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 224
	}
	for _, tt := range []struct {
		version uint16
		channel byte
		gus     bool
	}{
		{0x1301, 0, false},
		{0x1301, 8, false},
		{0x1301, 8, true},
		{0x1320, 8, false},
	} {
		for arg := byte(0); arg < 16; arg++ {
			t.Run(fmt.Sprintf("%04X/ch%d/gus%t/SA%X", tt.version, tt.channel, tt.gus, arg), func(t *testing.T) {
				data := referenceS3M(19, 0xa0|arg, sample)
				binary.LittleEndian.PutUint16(data[0x28:], tt.version)
				data[0x40] = tt.channel
				if tt.gus {
					binary.LittleEndian.PutUint16(data[0x128:], 2)
				}
				ref, actual := referencePCM(t, data)
				pan := func(pcm []float32) float64 {
					var left, right float64
					for i := 480; i < 960; i++ {
						left += math.Abs(float64(pcm[2*i]))
						right += math.Abs(float64(pcm[2*i+1]))
					}
					return right / (left + right)
				}
				want, got := pan(ref), pan(actual)
				if math.IsNaN(got) || math.Abs(got-want) > .015 {
					t.Errorf("OpenMPT pan %.4f, tracker %.4f", want, got)
				}
			})
		}
	}
}
