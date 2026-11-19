package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MVibratoMemoryWithoutVoice(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	data := referenceS3M(0, 0, sample)
	rows := []byte{0x80, 8, 0x4f, 0,
		0xa0, 0x40, 1, 21, 0, 0}
	rows = append(rows, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
	for tick := 6; tick < 12; tick++ {
		want := referenceTickFrequency(t, ref, tick)
		got := referenceTickFrequency(t, actual, tick)
		if math.Abs(got/want-1) > .005 {
			t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
		}
	}
}
