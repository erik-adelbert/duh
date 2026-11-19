package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MPanbrelloMemory(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	data := referenceS3M(0, 0, sample)
	data[0x33] = 0x30
	rows := []byte{0xa0, 0x40, 1, 25, 0x27, 0,
		0, 0x80, 25, 0, 0}
	rows = append(rows, make([]byte, 61)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	ref, actual := referencePCMWithChunks(t, data, "1", 360, 256)
	for _, ms := range []int{15, 75, 155, 195, 275, 315, 355} {
		ratio := func(pcm []float32) float64 {
			var left, right float64
			for i := ms * referenceRate / 1000; i < (ms+4)*referenceRate/1000; i++ {
				left += math.Abs(float64(pcm[2*i]))
				right += math.Abs(float64(pcm[2*i+1]))
			}
			return right / (left + right)
		}
		want, got := ratio(ref), ratio(actual)
		if math.IsNaN(got) || math.Abs(got-want) > .025 {
			t.Errorf("%dms: OpenMPT pan %.3f tracker %.3f", ms, want, got)
		}
	}
}
