package mod

import (
	"encoding/binary"
	"testing"
)

func TestLoadMPTMAcceptsFlexibleProfile(t *testing.T) {
	for _, tt := range []struct {
		name      string
		byteIndex int
		bit       byte
		check     func(ITCompatibility) bool
	}{
		{"flexible note-off", 12, 8, func(c ITCompatibility) bool { return c.oplFlexibleNoteOff }},
		{"channel pan restore", 12, 0x40, func(c ITCompatibility) bool { return c.itRestoreChannelPan }},
		{"duplicate check", 13, 1, func(c ITCompatibility) bool { return c.itDCTSampleAcrossInstruments }},
		{"OPL NNA", 13, 2, nil},
		{"OPL envelope no reset", 13, 0x20, func(c ITCompatibility) bool { return c.oplNoResetAtEnvelopeEnd }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := mptmPlaybackFixture()
			field := bytesIndex(t, b, ".FSM")
			end := field + 6 + 7
			binary.LittleEndian.PutUint16(b[field+4:], 14)
			data := append([]byte(nil), b[:end]...)
			extra := make([]byte, 7)
			extra[tt.byteIndex-7] = tt.bit
			data = append(data, extra...)
			data = append(data, b[end:]...)
			tail := len(data) - 4
			binary.LittleEndian.PutUint32(data[tail:], binary.LittleEndian.Uint32(data[tail:])+uint32(len(extra)))
			loaded, err := loadMPTM(data)
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil && !tt.check(loaded.Assets.ITCompatibility) {
				t.Fatal("expected projected behavior")
			}
		})
	}
}
