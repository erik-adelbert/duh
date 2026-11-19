package mod

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestMPTMIDIBankExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("..BM", 2, []byte{0, 0, 0x34, 0x12})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentMIDIBank, []uint16{0, 0x1234}) {
		t.Fatalf("MIDI banks: %v", ext.instrumentMIDIBank)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("..BM", 1, make([]byte, 2))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("..BM", 2, []byte{0, 0})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("..BM", 2, make([]byte, 4))...), mptExtensionField("..BM", 2, make([]byte, 4))...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTMIDIBankLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, bank := range []uint16{0, 1, 0x100, 16384, 65535} {
		values := make([]byte, 4)
		binary.LittleEndian.PutUint16(values[2:], bank)
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("..BM", 2, values)...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if bank == 0 && err != nil || bank != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported extended MIDI bank")) {
			t.Fatalf("bank %d: %v", bank, err)
		}
	}
}

func TestMPTMIDIChannelExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("..CM", 1, []byte{0, 17})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentMIDIChannel, []uint8{0, 17}) {
		t.Fatalf("MIDI channels: %v", ext.instrumentMIDIChannel)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("..CM", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("..CM", 1, []byte{0})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("..CM", 1, []byte{0, 0})...), mptExtensionField("..CM", 1, []byte{0, 0})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTMIDIChannelLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, channel := range []byte{0, 1, 17, 128} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("..CM", 1, []byte{0, channel})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if channel == 0 && err != nil || channel != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported extended MIDI channel")) {
			t.Fatalf("channel %d: %v", channel, err)
		}
	}
}

func TestMPTMIDIProgramExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("..PM", 1, []byte{0, 128})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentMIDIProgram, []uint8{0, 128}) {
		t.Fatalf("MIDI programs: %v", ext.instrumentMIDIProgram)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("..PM", 2, make([]byte, 4))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("..PM", 1, []byte{0})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("..PM", 1, []byte{0, 0})...), mptExtensionField("..PM", 1, []byte{0, 0})...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTMIDIProgramLoader(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, false)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, program := range []byte{0, 1, 128, 255} {
		data := append([]byte(nil), b[:bounds.propertiesStart+4]...)
		data = append(data, mptExtensionField("..PM", 1, []byte{0, program})...)
		data = append(data, b[bounds.propertiesStart+4:bounds.serializedStart]...)
		tail := len(data)
		data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
		data = binary.LittleEndian.AppendUint32(data, uint32(tail))
		_, err := loadMPTM(data)
		if program == 0 && err != nil || program != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported extended MIDI program")) {
			t.Fatalf("program %d: %v", program, err)
		}
	}
}
