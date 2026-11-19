package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestMPTSerializedBlockBounds(t *testing.T) {
	fixture := mptTestBlock("test", []mptTestEntry{{"a", []byte{1, 2}}, {"b", []byte{3, 4}}, {"c", []byte{5}}})
	// The fixture writes a fixed-width map pointer and two fixed-width map entries.
	mapPointer := 4 + len("test") + 6 + 2
	mapStart := int(binary.LittleEndian.Uint64(fixture[mapPointer:]) >> 2)
	secondStart := mapStart + 18 + 2
	for _, tt := range []struct {
		name  string
		start uint64
		want  string
	}{
		{"overlap", uint64(mapPointer + 8 + 1), "overlapping"},
		{"gap", uint64(mapPointer + 8 + 3), "unaccounted"},
		{"past map", uint64(mapStart + 1), "bounds"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := append([]byte(nil), fixture...)
			copy(b[secondStart:], mptTestAdaptive(tt.start, 2, 8))
			_, err := parseMPTSerializedBlock(b, "test")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
	for _, data := range [][]byte{append(append([]byte(nil), fixture...), 0), fixture[:len(fixture)-1]} {
		if _, err := parseMPTSerializedBlock(data, "test"); err == nil {
			t.Fatal("invalid serialized map accepted")
		}
	}
}

func TestMPTSerializedAdaptiveIntegers(t *testing.T) {
	for _, tt := range []struct {
		bits        int
		exponential bool
		length      int
		value       uint64
	}{
		{1, false, 1, 127}, {1, false, 2, 32767},
		{2, false, 1, 63}, {2, false, 4, 1<<30 - 1},
		{2, true, 1, 63}, {2, true, 2, 16383}, {2, true, 4, 1<<30 - 1}, {2, true, 8, 1<<62 - 1},
	} {
		data := mptTestAdaptive(tt.value, tt.bits, tt.length)
		if tt.exponential && tt.length == 4 {
			data[0] = data[0]&^3 | 2
		}
		pos := 0
		got, err := mptAdaptive(data, &pos, tt.bits, tt.exponential)
		if err != nil || got != tt.value || pos != len(data) {
			t.Fatalf("%+v: got %d at %d: %v", tt, got, pos, err)
		}
		for i := range data {
			pos = 0
			if _, err := mptAdaptive(data[:i], &pos, tt.bits, tt.exponential); err == nil {
				t.Fatalf("%+v: accepted truncated integer at %d", tt, i)
			}
		}
	}
}
