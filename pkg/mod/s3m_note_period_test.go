package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MNotePeriod(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, note := range []byte{0x37, 0x43, 0x51, 0x5a, 0x66} {
		for _, speed := range []uint32{6000, 8363, 8529, 12000} {
			t.Run(fmt.Sprintf("%02X/%d", note, speed), func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				data[0x153] = note
				data[0x11f] = 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				binary.LittleEndian.PutUint32(data[0x120:], speed)
				ref, actual := referencePCMWithChunks(t, data, "1", 400, 256)
				want, got := s3mSustainedFrequency(t, ref), s3mSustainedFrequency(t, actual)
				if math.Abs(got/want-1) > .0001 {
					t.Errorf("OpenMPT %.4f Hz, tracker %.4f Hz", want, got)
				}
			})
		}
	}
}
