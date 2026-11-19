package mod

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestOpenMPTDoublePortamentoSupportedRows(t *testing.T) {
	tests := []struct {
		name string
		row  []byte
	}{
		{"effect slide", []byte{0x81, 8, 6, 8, 0}},
		{"volume slide", []byte{0x81, 4, 109, 0}},
		{"volume tone and pitch", []byte{0x81, 13, 64, 197, 6, 8, 0}},
		{"volume tone and pitch memory", []byte{0x81, 13, 64, 193, 6, 8, 0}},
		{"volume tone and tone", []byte{0x81, 13, 64, 197, 7, 8, 0}},
		{"volume tone and tone memory", []byte{0x81, 13, 64, 193, 7, 8, 0}},
		{"delayed volume tone", []byte{0x81, 13, 64, 197, 19, 0xd1, 0}},
	}
	for _, tt := range tests {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/compatible=%t", tt.name, compatible), func(t *testing.T) {
				b := mptCarryAfterNoteOffFixture(t, false, false, 255)
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				length := int(binary.LittleEndian.Uint16(b[pattern:]))
				binary.LittleEndian.PutUint16(b[pattern+2:], 32)
				clear(b[pattern+8 : pattern+8+length])
				rows := append([]byte{0x81, 3, 60, 2, 0}, tt.row...)
				copy(b[pattern+8:], rows)
				b[bytesIndex(t, b, ".FSM")+6+mptITDoublePortamentoSlidesBit/8] |= 1 << (mptITDoublePortamentoSlidesBit % 8)
				if compatible {
					b[44] |= 0x20
				} else {
					b[44] &^= 0x20
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
				assertMPTPCMCorrelation(t, ref, actual, 80, 110)
			})
		}
	}
}
