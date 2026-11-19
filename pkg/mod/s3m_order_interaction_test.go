package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MJumpBreakAcrossSkip(t *testing.T) {
	sample := make([]byte, 700)
	for i := range sample {
		sample[i] = 224
	}
	data := referenceS3M(0, 0, sample)
	binary.LittleEndian.PutUint16(data[0x20:], 3)
	binary.LittleEndian.PutUint16(data[0x24:], 2)
	data[0x60], data[0x61], data[0x62] = 0, 254, 1
	binary.LittleEndian.PutUint16(data[0x63:], 0x10)
	binary.LittleEndian.PutUint16(data[0x65:], 0x15)
	binary.LittleEndian.PutUint16(data[0x67:], 0x1a)
	data[0x41] = 1
	binary.LittleEndian.PutUint32(data[0x100+0x20:], 48000)
	first := append([]byte{0xa0, 0x40, 1, 2, 1, 0x81, 3, 5, 0}, make([]byte, 63)...)
	second := append(make([]byte, 5), []byte{0x20, 0x40, 1, 0}...)
	second = append(second, make([]byte, 58)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(first)))
	copy(data[0x152:], first)
	binary.LittleEndian.PutUint16(data[0x1a0:], uint16(len(second)))
	copy(data[0x1a2:], second)
	ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		var onsets []int
		active := false
		for i := 0; i < len(pcm)/2; i++ {
			next := math.Abs(float64(pcm[2*i])) > .001
			if next && !active {
				onsets = append(onsets, i)
			}
			active = next
		}
		if len(onsets) < 2 || onsets[1] < 5700 || onsets[1] > 5820 {
			t.Errorf("%s: expected jump to row 5 near 120ms, got %v", name, onsets)
		}
	}
}
