package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func testITPitchFixture(rows []byte, speed byte, linear, legacy bool) []byte {
	b := testIT()
	b[50] = speed
	b[256+18] |= 0x10
	binary.LittleEndian.PutUint32(b[256+56:], 3840)
	for i := range 4096 {
		value := int16(math.Sin(float64(i)*2*math.Pi/8) * 16000)
		binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
	}
	if linear {
		b[44] |= 8
	}
	if legacy {
		binary.LittleEndian.PutUint16(b[40:], 0x217)
		binary.LittleEndian.PutUint16(b[42:], 0x200)
	}
	packed := append(append([]byte(nil), rows...), make([]byte, 64)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	return b
}

func referenceTickFrequency(t *testing.T, pcm []float32, tick int) float64 {
	t.Helper()
	first, last, count := 0.0, 0.0, 0
	for i := tick*960 + 100; i < (tick+1)*960-100; i++ {
		before, after := float64(pcm[2*(i-1)]), float64(pcm[2*i])
		if before <= 0 && after > 0 {
			pos := float64(i-1) - before/(after-before)
			if count == 0 {
				first = pos
			}
			last = pos
			count++
		}
	}
	if count < 2 || last <= first {
		t.Fatalf("tick %d: insufficient zero crossings for pitch comparison", tick)
	}
	return float64(count-1) * referenceRate / (last - first)
}
