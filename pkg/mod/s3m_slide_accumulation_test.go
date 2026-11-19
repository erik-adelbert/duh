package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MPitchSlideAccumulation(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, profile := range []struct {
		name     string
		version  uint16
		reserved uint16
	}{
		{"st3", 0x1301, 0},
		{"it", 0x3212, 0},
		{"old schism", 0x4050, 0},
		{"recent schism", 0x4fff, 4201},
	} {
		for _, effect := range []struct {
			name    string
			command byte
		}{
			{"up", 6},
			{"down", 5},
		} {
			t.Run(profile.name+"/"+effect.name, func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				binary.LittleEndian.PutUint16(data[0x28:], profile.version)
				binary.LittleEndian.PutUint16(data[0x36:], profile.reserved)
				data[0x31], data[0x11f] = 16, 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				binary.LittleEndian.PutUint32(data[0x120:], 8529)
				rows := []byte{0xa0, 0x40, 1, effect.command, 4, 0}
				for i := 0; i < 3; i++ {
					rows = append(rows, 0x80, effect.command, 0, 0)
				}
				rows = append(rows, make([]byte, 60)...)
				binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
				copy(data[0x152:], rows)
				ref, actual := referencePCMWithChunks(t, data, "1", 1280, 256)
				for _, tick := range []int{1, 7, 15, 23, 31, 39, 47, 55, 63} {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(got/want-1) > .002 {
						t.Errorf("tick %d: OpenMPT %.2f Hz, tracker %.2f Hz", tick, want, got)
					}
				}
			})
		}
	}
}
