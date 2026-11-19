package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MArpeggioAfterPitchEffects(t *testing.T) {
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
		{"recent schism", 0x4fff, 4201},
	} {
		for _, effect := range []struct {
			name string
			rows []byte
		}{
			{"pitch slide", []byte{0xa0, 0x40, 1, 6, 8, 0, 0x80, 10, 0x47, 0}},
			{"tone portamento", []byte{0x20, 0x40, 1, 0, 0xa0, 0x50, 0, 7, 16, 0, 0x80, 10, 0x47, 0}},
		} {
			t.Run(profile.name+"/"+effect.name, func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				binary.LittleEndian.PutUint16(data[0x28:], profile.version)
				binary.LittleEndian.PutUint16(data[0x36:], profile.reserved)
				data[0x31], data[0x11f] = 3, 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				rows := append(append([]byte(nil), effect.rows...), make([]byte, 64)...)
				binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
				copy(data[0x152:], rows)
				ref, actual := referencePCMWithChunks(t, data, "1", 180, 256)
				for tick := 3; tick < 9; tick++ {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(got/want-1) > .003 {
						t.Errorf("tick %d: OpenMPT %.2f Hz, tracker %.2f Hz", tick, want, got)
					}
				}
			})
		}
	}
}
