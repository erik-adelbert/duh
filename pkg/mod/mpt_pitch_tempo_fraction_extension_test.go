package mod

import (
	"encoding/binary"
	"reflect"
	"strings"
	"testing"
)

func TestMPTPitchTempoFractionExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("PTTF", 2, []byte{0, 0, 0x34, 0x12})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentPitchTempoFraction, []uint16{0, 0x1234}) {
		t.Fatalf("pitch-to-tempo fractions: %v", ext.instrumentPitchTempoFraction)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("PTTF", 1, make([]byte, 2))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("PTTF", 2, []byte{0, 0})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("PTTF", 2, make([]byte, 4))...), mptExtensionField("PTTF", 2, make([]byte, 4))...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTPitchTempoFractionLoader(t *testing.T) {
	for _, fraction := range []uint16{0, 1, 256, 65535} {
		values := make([]byte, 4)
		binary.LittleEndian.PutUint16(values[2:], fraction)
		data := mptExtraExtensionsFixture(t, mptExtensionField("PTTF", 2, values), nil)
		_, err := loadMPTM(data)
		if fraction == 0 && err != nil || fraction != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported fractional pitch-to-tempo lock")) {
			t.Fatalf("fraction %d: %v", fraction, err)
		}
	}
}

func TestMPTPitchTempoLockExtension(t *testing.T) {
	data := append([]byte("XTPM"), mptExtensionField("LTTP", 2, []byte{0, 0, 0x34, 0x12})...)
	ext, err := parseMPTExtensions(data, 2)
	if err != nil {
		t.Fatal(err)
	}
	clear(data)
	if !reflect.DeepEqual(ext.instrumentPitchTempoLock, []uint16{0, 0x1234}) {
		t.Fatalf("pitch-to-tempo lock values: %v", ext.instrumentPitchTempoLock)
	}
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"width", append([]byte("XTPM"), mptExtensionField("LTTP", 1, make([]byte, 2))...), "invalid size"},
		{"truncated", append([]byte("XTPM"), mptExtensionField("LTTP", 2, []byte{0, 0})...), "truncated"},
		{"duplicate", append(append([]byte("XTPM"), mptExtensionField("LTTP", 2, make([]byte, 4))...), mptExtensionField("LTTP", 2, make([]byte, 4))...), "duplicate"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMPTExtensions(tt.data, 2); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestMPTPitchTempoLockLoader(t *testing.T) {
	for _, tempo := range []uint16{0, 1, 256, 65535} {
		values := make([]byte, 4)
		binary.LittleEndian.PutUint16(values[2:], tempo)
		data := mptExtraExtensionsFixture(t, mptExtensionField("LTTP", 2, values), nil)
		_, err := loadMPTM(data)
		if tempo == 0 && err != nil || tempo != 0 && (err == nil || !strings.Contains(err.Error(), "instrument 2 uses unsupported pitch-to-tempo lock")) {
			t.Fatalf("tempo %d: %v", tempo, err)
		}
	}
}
