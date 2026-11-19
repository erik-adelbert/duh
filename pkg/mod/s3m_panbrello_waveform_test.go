package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MPanbrelloWaveform(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	for _, waveform := range []byte{0, 1, 2, 3} {
		t.Run(fmt.Sprint(waveform), func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x33] = 0x30
			rows := []byte{0xa0, 0x40, 1, 19, 0x50 | waveform, 0,
				0x80, 25, 0x47, 0}
			rows = append(rows, make([]byte, 62)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
			for _, ms := range []int{159, 179, 199, 219, 239} {
				ratio := func(pcm []float32) float64 {
					var left, right float64
					for i := ms * referenceRate / 1000; i < (ms+1)*referenceRate/1000; i++ {
						left += math.Abs(float64(pcm[2*i]))
						right += math.Abs(float64(pcm[2*i+1]))
					}
					return right / (left + right)
				}
				want, got := ratio(ref), ratio(actual)
				if math.IsNaN(got) || math.Abs(got-want) > .035 {
					t.Errorf("%dms: OpenMPT pan %.3f tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
