package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MSpecialFlow(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"pattern loop", []byte{
			0xa0, 0x40, 1, 19, 0xb0, 0,
			0xa0, 0x40, 1, 19, 0xb2, 0,
			0x20, 0x40, 1, 0,
		}},
		{"pattern delay and memory", []byte{
			0xa0, 0x40, 1, 19, 0xe1, 0,
			0xa0, 0x40, 1, 19, 0, 0,
			0x20, 0x40, 1, 0,
		}},
		{"fine pattern delay", []byte{
			0xa0, 0x40, 1, 19, 0x63, 0,
			0xa0, 0x40, 1, 19, 0x61, 0,
			0x20, 0x40, 1, 0,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := make([]byte, 700)
			for i := range sample {
				sample[i] = 224
			}
			data := referenceS3M(0, 0, sample)
			binary.LittleEndian.PutUint32(data[0x100+0x20:], 48000)
			rows := append(append([]byte(nil), tt.rows...), make([]byte, 61)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			ref, actual := referencePCMWithChunks(t, data, "1", 1000, 256)
			onsets := func(pcm []float32) []int {
				var found []int
				active := false
				for i := 0; i < len(pcm)/2; i++ {
					next := math.Abs(float64(pcm[2*i])) > .001
					if next && !active {
						found = append(found, i)
					}
					active = next
				}
				return found
			}
			want, got := onsets(ref), onsets(actual)
			if len(want) != len(got) {
				t.Fatalf("onsets: OpenMPT %v tracker %v", want, got)
			}
			for i := range want {
				if delta := got[i] - want[i]; delta < -20 || delta > 20 {
					t.Errorf("onset %d: OpenMPT %d tracker %d", i, want[i], got[i])
				}
			}
		})
	}
}
