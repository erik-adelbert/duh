package mod

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestOpenMPTFilterEnvelopeControlAfterStart(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"pause", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x7b, 0}},
		{"resume", []byte{0x81, 11, 60, 2, 19, 0x7b, 0, 0x81, 8, 19, 0x7c, 0}},
		{"pause resume", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x7b, 0, 0x81, 8, 19, 0x7c, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/enabled=%t", tt.name, enabled), func(t *testing.T) {
				b := mptStoppedFilterEnvelopeFixture(t, enabled, 80, 40)
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				length := int(binary.LittleEndian.Uint16(b[pattern:]))
				binary.LittleEndian.PutUint16(b[pattern+2:], 32)
				clear(b[pattern+8 : pattern+8+length])
				copy(b[pattern+8:], tt.rows)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 180)
				assertMPTPCMCorrelation(t, ref, actual, 90, 150)
			})
		}
	}
}
