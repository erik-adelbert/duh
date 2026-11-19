package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MPitchSlides(t *testing.T) {
	tests := []struct {
		name        string
		command     byte
		param       byte
		priorEffect byte
		priorParam  byte
		c5speed     uint32
	}{
		{"up", 6, 8, 0, 0, 0},
		{"down", 5, 8, 0, 0, 0},
		{"fine up", 6, 0xf3, 0, 0, 0},
		{"fine down", 5, 0xf3, 0, 0, 0},
		{"extra fine up", 6, 0xe3, 0, 0, 0},
		{"extra fine down", 5, 0xe3, 0, 0, 0},
		{"memory from speed", 6, 0, 1, 3, 0},
		{"memory from volume slide", 5, 0, 4, 0x10, 0},
		{"memory from opposite slide", 6, 0, 5, 8, 0},
		{"up tuned sample", 6, 8, 0, 0, 12000},
		{"down tuned sample", 5, 8, 0, 0, 6000},
	}
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = byte(128 + int(100*math.Sin(float64(i)*2*math.Pi/8)))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x31] = 3
			data[0x100+0x1f] = 1
			if tt.c5speed != 0 {
				binary.LittleEndian.PutUint32(data[0x100+0x20:], tt.c5speed)
			}
			binary.LittleEndian.PutUint32(data[0x100+0x18:], uint32(len(sample)))
			rows := []byte{0xa0, 0x40, 1, tt.priorEffect, tt.priorParam, 0,
				0x80, tt.command, tt.param, 0,
				0x80, tt.command, 0, 0}
			rows = append(rows, make([]byte, 61)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			ref, actual := referencePCMWithChunks(t, data, "1", 180, 256)
			for tick := 0; tick < 9; tick++ {
				want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
				if math.Abs(got/want-1) > .006 {
					t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
				}
			}
		})
	}
}
