package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestMPTSustainPortamentoInheritedPingPongValidation(t *testing.T) {
	b := mptSustainPortaFixture(t, true, false, true, 1)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x50
	binary.LittleEndian.PutUint32(b[sample+52:], 240)
	binary.LittleEndian.PutUint32(b[sample+56:], 320)
	binary.LittleEndian.PutUint32(b[sample+68:], 129)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "one-sample ping-pong sustain") {
		t.Fatalf("one-sample inherited ping-pong loop: %v", err)
	}

	binary.LittleEndian.PutUint32(b[sample+68:], 193)
	flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITPingPongMode)/8
	b[flag] &^= 1 << (mptITPingPongMode % 8)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "non-IT ping-pong mode") {
		t.Fatalf("non-IT inherited ping-pong loop: %v", err)
	}
}
