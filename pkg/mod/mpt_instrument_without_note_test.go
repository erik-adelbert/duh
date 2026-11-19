package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptInstrumentWithoutNoteFixture(t *testing.T, rows []byte, enabled bool, nna byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, 1, 60, false)
	b[50] = 3
	b[bytesIndex(t, b, "...C")+6] = 3
	flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITInstrWithoutNote)/8
	mask := byte(1 << (mptITInstrWithoutNote % 8))
	b[flag] &^= mask
	if enabled {
		b[flag] |= mask
	}
	first := int(binary.LittleEndian.Uint32(b[194:]))
	second := int(binary.LittleEndian.Uint32(b[198:]))
	b[first+24], b[first+304], b[second+17], b[second+304] = 64, 0, nna, 0
	for i := range 120 {
		b[first+65+i*2] = 1
	}
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	data := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		value := 15000 * (1 - float64(i)/4096) * math.Sin(2*math.Pi*float64(i)/32)
		binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(value)))
	}
	return mptPatternRowsFixture(t, b, rows)
}

func TestOpenMPTMInstrumentWithoutNote(t *testing.T) {
	tests := []struct {
		name   string
		before []byte
		instr  byte
	}{
		{"change-active", []byte{0x81, 3, 60, 2, 0}, 1},
		{"same-active", []byte{0x81, 3, 60, 2, 0}, 2},
		{"change-after-cut", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 254, 0}, 1},
		{"same-after-cut", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 254, 0}, 2},
		{"change-after-off", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 255, 0}, 1},
		{"same-after-off", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 255, 0}, 2},
		{"none-after-off", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 255, 0}, 0},
		{"change-after-fade", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 253, 0}, 1},
		{"same-after-fade", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 253, 0}, 2},
		{"none-after-fade", []byte{0x81, 3, 60, 2, 0, 0x81, 1, 253, 0}, 0},
		{"change-after-end", []byte{0x81, 3, 60, 2, 0, 0, 0, 0, 0, 0, 0, 0}, 1},
		{"same-after-end", []byte{0x81, 3, 60, 2, 0, 0, 0, 0, 0, 0, 0, 0}, 2},
		{"change-again-after-end", []byte{0x81, 3, 60, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0x81, 2, 1, 0}, 2},
	}

	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, nna := range []byte{0, 1} {
				t.Run(fmt.Sprintf("%s/flag=%t/nna=%d", tt.name, enabled, nna), func(t *testing.T) {
					rows := append(append([]byte(nil), tt.before...), 0x81, 2, tt.instr, 0)
					b := mptInstrumentWithoutNoteFixture(t, rows, enabled, nna)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 750)
					for ms := 20; ms < 740; ms += 10 {
						if ms%60 == 0 {
							continue
						}
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(got-want) > .003 {
							t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}
