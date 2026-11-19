package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptOffsetInstrumentFixture(t *testing.T, rows []byte, enabled, instrumentWithoutNote bool) []byte {
	t.Helper()
	b := mptInstrumentWithoutNoteFixture(t, rows, instrumentWithoutNote, 0)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	data := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		frequency := .02
		if i >= 256 && i < 512 {
			frequency = .11
		}
		binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(15000*math.Sin(float64(i)*2*math.Pi*frequency))))
	}
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01320400)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	flags := make([]byte, 16)
	copy(flags, b[field+6:field+6+length])
	if enabled {
		flags[mptITOffsetWithInstrumentBit/8] |= 1 << (mptITOffsetWithInstrumentBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTOffsetWithInstrument(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"same instrument", []byte{0x81, 3, 60, 2, 0, 0x81, 10, 2, 15, 1, 0}},
		{"changed instrument", []byte{0x81, 3, 60, 2, 0, 0x81, 10, 1, 15, 1, 0}},
		{"memory", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 15, 1, 0, 0x81, 10, 2, 15, 0, 0}},
		{"without instrument", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 15, 1, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, instrumentWithoutNote := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/insOnly=%t", tt.name, enabled, instrumentWithoutNote), func(t *testing.T) {
					b := mptOffsetInstrumentFixture(t, tt.rows, enabled, instrumentWithoutNote)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
					start := 80
					if tt.name == "memory" {
						start = 140
					}
					assertMPTPCMCorrelation(t, ref, actual, start, start+30)
				})
			}
		}
	}
}
