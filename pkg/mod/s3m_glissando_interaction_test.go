package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MGlissandoTransitions(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"disable", []byte{0xa0, 0x40, 1, 19, 0x11, 0,
			0xa0, 0x50, 0, 7, 16, 0, 0x80, 19, 0x10, 0, 0x80, 7, 0, 0}},
		{"recall", []byte{0xa0, 0x40, 1, 19, 0x11, 0,
			0xa0, 0x50, 0, 7, 16, 0, 0x80, 19, 0, 0, 0x80, 7, 0, 0}},
		{"pitch slide", []byte{0xa0, 0x40, 1, 19, 0x11, 0,
			0xa0, 0x50, 0, 7, 16, 0, 0x80, 6, 8, 0, 0x80, 6, 0, 0}},
		{"vibrato", []byte{0xa0, 0x40, 1, 19, 0x11, 0,
			0xa0, 0x50, 0, 7, 16, 0, 0x80, 8, 0x47, 0, 0x80, 8, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := make([]byte, 4096)
			for i := range sample {
				sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
			}
			data := referenceS3M(0, 0, sample)
			data[0x31] = 3
			data[0x100+0x1f] = 1
			binary.LittleEndian.PutUint32(data[0x100+0x18:], uint32(len(sample)))
			rows := append(append([]byte(nil), tt.rows...), make([]byte, 60)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
			for tick := 0; tick < 12; tick++ {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .006 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
