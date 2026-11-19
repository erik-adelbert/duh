package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MGlissando(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, tt := range []struct {
		name        string
		start, goal byte
	}{
		{"up", 0x40, 0x50},
		{"down", 0x50, 0x40},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprint(enabled), func(t *testing.T) {
					data := referenceS3M(0, 0, sample)
					data[0x31] = 3
					data[0x100+0x1f] = 1
					binary.LittleEndian.PutUint32(data[0x100+0x18:], uint32(len(sample)))
					arg := byte(0x10)
					if enabled {
						arg++
					}
					rows := []byte{0xa0, tt.start, 1, 19, arg, 0,
						0xa0, tt.goal, 0, 7, 16, 0,
						0x80, 7, 0, 0, 0x80, 7, 0, 0}
					rows = append(rows, make([]byte, 60)...)
					binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
					copy(data[0x152:], rows)
					ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
					for tick := 0; tick < 12; tick++ {
						want := referenceTickFrequency(t, ref, tick)
						got := referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .005 {
							t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
						}
					}
				})
			}
		})
	}
}
