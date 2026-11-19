package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MGlissandoProfiles(t *testing.T) {
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
		for _, direction := range []struct {
			name        string
			start, goal byte
		}{
			{"up", 0x37, 0x47},
			{"down", 0x47, 0x37},
		} {
			t.Run(profile.name+"/"+direction.name, func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				binary.LittleEndian.PutUint16(data[0x28:], profile.version)
				binary.LittleEndian.PutUint16(data[0x36:], profile.reserved)
				data[0x31], data[0x11f] = 3, 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				binary.LittleEndian.PutUint32(data[0x120:], 8529)
				rows := []byte{0xa0, direction.start, 1, 19, 0x11, 0,
					0xa0, direction.goal, 0, 7, 24, 0,
					0x80, 7, 0, 0, 0x80, 6, 8, 0}
				rows = append(rows, make([]byte, 60)...)
				binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
				copy(data[0x152:], rows)
				ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
				for tick := 0; tick < 12; tick++ {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(got/want-1) > .003 {
						t.Errorf("tick %d: OpenMPT %.2f Hz, tracker %.2f Hz", tick, want, got)
					}
				}
			})
		}
	}
}
