package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MNotePitchProfiles(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, tt := range []struct {
		name     string
		version  uint16
		reserved uint16
	}{
		{"st3", 0x1301, 0},
		{"it", 0x3212, 0},
		{"old schism", 0x4050, 0},
		{"recent schism", 0x4fff, 4201},
	} {
		for _, note := range []byte{0x37, 0x43, 0x51, 0x5a} {
			t.Run(fmt.Sprintf("%s/%02X", tt.name, note), func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				binary.LittleEndian.PutUint16(data[0x28:], tt.version)
				binary.LittleEndian.PutUint16(data[0x36:], tt.reserved)
				data[0x153] = note
				data[0x11f] = 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				binary.LittleEndian.PutUint32(data[0x120:], 8529)
				ref, actual := referencePCMWithChunks(t, data, "1", 400, 256)
				want, got := s3mSustainedFrequency(t, ref), s3mSustainedFrequency(t, actual)
				if math.Abs(got/want-1) > .0001 {
					t.Errorf("OpenMPT %.4f Hz, tracker %.4f Hz", want, got)
				}
			})
		}
	}
}
