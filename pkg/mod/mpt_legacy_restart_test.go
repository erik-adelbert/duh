package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestMPTLegacyRestartField(t *testing.T) {
	for _, tt := range []struct {
		data []byte
		ok   bool
	}{
		{[]byte{0, 0}, true},
		{[]byte{3, 0}, true},
		{nil, false},
		{[]byte{1}, false},
		{[]byte{1, 0, 0, 0}, false},
	} {
		data := append([]byte("STPM"), mptExtensionField("..PR", uint16(len(tt.data)), tt.data)...)
		ext, err := parseMPTExtensions(data, 0)
		if !tt.ok {
			if err == nil || !strings.Contains(err.Error(), "invalid size") {
				t.Fatalf("width %d: %v", len(tt.data), err)
			}
			continue
		}
		if err != nil || ext.legacyRestart == nil || *ext.legacyRestart != binary.LittleEndian.Uint16(tt.data) {
			t.Fatalf("restart %v: %+v, %v", tt.data, ext.legacyRestart, err)
		}
	}
}

func TestMPTLegacyRestartFallbackAndOverride(t *testing.T) {
	legacy := mptSequenceDefaults{restart: 3, tempo: 1250000, speed: 6}
	zero := uint16(0)
	for _, tt := range []struct {
		seq  mptSequence
		want uint16
	}{
		{mptSequence{orders: []uint16{0, 0, 0, 0}}, 3},
		{mptSequence{orders: []uint16{0, 0, 0, 0}, restart: &zero}, 0},
	} {
		got, err := resolveMPTSequenceDefaults(tt.seq, legacy)
		if err != nil || got.restart != tt.want {
			t.Fatalf("restart=%d, error %v, want %d", got.restart, err, tt.want)
		}
	}
}
