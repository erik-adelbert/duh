package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MRandomVibratoNoteReset(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, waveform := range []byte{3, 7} {
		t.Run(fmt.Sprint(waveform), func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x31] = 3
			data[0x100+0x1f] = 1
			binary.LittleEndian.PutUint32(data[0x100+0x18:], uint32(len(sample)))
			rows := []byte{0xa0, 0x40, 1, 19, 0x30 | waveform, 0,
				0x80, 8, 0x4f, 0,
				0xa0, 0x40, 1, 8, 0, 0,
				0x80, 8, 0, 0}
			rows = append(rows, make([]byte, 60)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
			for tick := 3; tick < 12; tick++ {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .006 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
