package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTQ00(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, opl := range []bool{false, true} {
			t.Run(fmt.Sprintf("compatible=%t/opl=%t", compatible, opl), func(t *testing.T) {
				b := mptmPlaybackFixture()
				b[50] = 2
				b[bytesIndex(t, b, "..MT")+6] = 0
				flags := bytesIndex(t, b, ".FSM") + 6
				if !compatible {
					b[flags+2] &^= 0x80
				}
				for i := range 4096 {
					value := int16(0)
					if i < 64 {
						value = 16000
					}
					binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
				}
				header := int(binary.LittleEndian.Uint32(b[198:]))
				patch := int(binary.LittleEndian.Uint32(b[header+72:]))
				copy(b[patch:patch+12], percussiveOPLPatch[:])
				instrument := byte(1)
				if opl {
					instrument = 2
				}
				rows := []byte{0x81, 3, 60, instrument, 0, 0x81, 8, 17, 0, 0, 0x81, 8, 17, 0, 0}
				pattern := int(binary.LittleEndian.Uint32(b[202:]))
				packed := append(rows, make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[pattern:], uint16(len(packed)))
				copy(b[pattern+8:], packed)
				loaded, err := loadMPTM(b)
				if err != nil {
					t.Fatal(err)
				}
				if loaded.Assets.ITCompatibility.mptITRetrigger != compatible {
					t.Fatal("retrig compatibility flag was not projected")
				}
				ref := referenceTransition(t, b)
				w := NewTracker(loaded.Assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 2, 125, -1
				actual := make([]float32, 5760*2)
				if w.Read(actual, 5760) != 5760 {
					t.Fatal("short render")
				}
				for tick := 2; tick < 6; tick++ {
					want := rmsWindow(ref, tick*20+2, tick*20+7) / rmsWindow(ref, 62, 67)
					got := rmsWindow(actual, tick*20+2, tick*20+7) / rmsWindow(actual, 62, 67)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .25 {
						t.Errorf("tick %d relative RMS: OpenMPT %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestMPTQ00Validation(t *testing.T) {
	for _, row := range [][]byte{
		{0x81, 9, 60, 17, 0, 0},
		{0x81, 10, 1, 17, 0, 0},
		{0x81, 12, 64, 17, 0, 0},
	} {
		if _, err := unpackITPatternMode(row, 1, true); err != nil {
			t.Fatalf("Q00 combination %v: %v", row, err)
		}
	}
	for _, row := range [][]byte{
		{0x81, 9, 253, 17, 1, 0},
		{0x81, 12, 105, 17, 1, 0},
		{0x81, 9, 253, 17, 0, 0},
		{0x81, 9, 254, 17, 0, 0},
		{0x81, 9, 255, 17, 0, 0},
		{0x81, 12, 105, 17, 0, 0},
	} {
		if _, err := unpackITPatternMode(row, 1, true); err == nil {
			t.Fatalf("Q01 unsupported combination %v accepted", row)
		}
	}
}
