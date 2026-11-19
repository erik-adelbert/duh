package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MPitchEffectProfiles(t *testing.T) {
	profiles := []struct {
		name     string
		version  uint16
		reserved uint16
	}{
		{"st3", 0x1301, 0},
		{"it", 0x3212, 0},
		{"old schism", 0x4050, 0},
		{"recent schism", 0x4fff, 4201},
	}
	effects := []struct {
		name string
		rows []byte
	}{
		{"up", []byte{0xa0, 0x40, 1, 6, 32, 0, 0x80, 6, 0, 0}},
		{"down", []byte{0xa0, 0x40, 1, 5, 32, 0, 0x80, 5, 0, 0}},
		{"fine up", []byte{0xa0, 0x40, 1, 6, 0xff, 0, 0x80, 6, 0, 0}},
		{"fine down", []byte{0xa0, 0x40, 1, 5, 0xff, 0, 0x80, 5, 0, 0}},
		{"extra fine", []byte{0xa0, 0x40, 1, 6, 0xef, 0, 0x80, 6, 0, 0}},
		{"tone up", []byte{0x20, 0x40, 1, 0, 0xa0, 0x50, 0, 7, 32, 0, 0x80, 7, 0, 0}},
		{"tone down", []byte{0x20, 0x50, 1, 0, 0xa0, 0x40, 0, 7, 32, 0, 0x80, 7, 0, 0}},
		{"vibrato", []byte{0xa0, 0x40, 1, 8, 0x47, 0, 0x80, 8, 0, 0}},
		{"fine vibrato", []byte{0xa0, 0x40, 1, 21, 0x47, 0, 0x80, 21, 0, 0}},
		{"finetune", []byte{0xa0, 0x40, 1, 19, 0x27, 0, 0x80, 6, 0x20, 0}},
		{"arpeggio", []byte{0xa0, 0x40, 1, 10, 0x47, 0, 0x80, 10, 0, 0}},
	}
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, profile := range profiles {
		for _, effect := range effects {
			t.Run(fmt.Sprintf("%s/%s", profile.name, effect.name), func(t *testing.T) {
				data := referenceS3M(0, 0, sample)
				binary.LittleEndian.PutUint16(data[0x28:], profile.version)
				binary.LittleEndian.PutUint16(data[0x36:], profile.reserved)
				data[0x31], data[0x11f] = 3, 1
				binary.LittleEndian.PutUint32(data[0x118:], uint32(len(sample)))
				binary.LittleEndian.PutUint32(data[0x120:], 8529)
				rows := append(append([]byte(nil), effect.rows...), make([]byte, 64)...)
				binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
				copy(data[0x152:], rows)
				ref, actual := referencePCMWithChunks(t, data, "1", 180, 256)
				for tick := 0; tick < 6; tick++ {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(got/want-1) > .002 {
						t.Errorf("tick %d: OpenMPT %.2f Hz, tracker %.2f Hz", tick, want, got)
					}
				}
			})
		}
	}
}
