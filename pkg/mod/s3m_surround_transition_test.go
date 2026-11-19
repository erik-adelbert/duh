package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MSurroundToPanning(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	data := referenceS3M(0, 0, sample)
	rows := []byte{0xa0, 0x40, 1, 19, 0x91, 0,
		0x80, 19, 0x8c, 0}
	rows = append(rows, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
	for _, ms := range []int{55, 175} {
		ratio := func(pcm []float32) float64 {
			var left, right float64
			for i := ms * referenceRate / 1000; i < (ms+4)*referenceRate/1000; i++ {
				left += float64(pcm[2*i])
				right += float64(pcm[2*i+1])
			}
			return right / (math.Abs(left) + math.Abs(right))
		}
		want, got := ratio(ref), ratio(actual)
		if math.IsNaN(got) || math.Abs(got-want) > .04 {
			t.Errorf("%dms: OpenMPT pan %.3f tracker %.3f", ms, want, got)
		}
	}
}
