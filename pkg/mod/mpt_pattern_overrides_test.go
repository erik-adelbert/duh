package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestMPTSerializedPatternData(t *testing.T) {
	patterns := []PatternAsset{{Rows: [][]ModCommand{
		{{Note: 1}, {Note: 2}}, {{Note: 3}, {Note: 4}}, {{Note: 5}, {Note: 6}},
	}}}
	data := []byte{0x81, 0x23, 0xfb, 1, 25, 0, 0x01, 0, 0x82, 0x43, 0xfc, 2, 2, 10, 11, 0}
	block := mptTestBlock("mptPc", []mptTestEntry{
		{"num", []byte{1, 0}},
		{"\x00\x00", mptTestBlock("mptP", []mptTestEntry{{"data", data}, {"COL", []byte{1, 2, 3, 4}}})},
	})
	bypassed, err := applyMPTPatternOverrides(block, patterns, 2)
	if err != nil || !bypassed {
		t.Fatalf("bypassed %t, error %v", bypassed, err)
	}
	for _, cell := range [][2]int{{0, 0}, {1, 0}, {2, 1}} {
		if got := patterns[0].Rows[cell[0]][cell[1]]; got != (ModCommand{}) {
			t.Fatalf("cell %v not replaced: %+v", cell, got)
		}
	}
	if patterns[0].Rows[1][1].Note != 4 || patterns[0].Rows[2][0].Note != 5 {
		t.Fatal("unrelated cells were modified")
	}
}

func TestMPTSerializedPatternRejection(t *testing.T) {
	for _, tt := range []struct {
		name   string
		fields []mptTestEntry
		want   string
	}{
		{"incomplete signature", []mptTestEntry{{"RPB.", binary.LittleEndian.AppendUint32(nil, 8)}}, "invalid pattern signature"},
		{"swing", []mptTestEntry{{"SWNG", []byte{0}}}, "invalid pattern tempo swing size"},
		{"unknown", []mptTestEntry{{"unknown", []byte{0}}}, "unsupported serialized pattern field"},
		{"color", []mptTestEntry{{"COL", []byte{0}}}, "invalid color"},
		{"truncated mask", []mptTestEntry{{"data", []byte{0x81}}}, "truncated"},
		{"truncated value", []mptTestEntry{{"data", []byte{0x81, 1}}}, "truncated"},
		{"truncated extra", []mptTestEntry{{"data", []byte{0x81, 0x41, 0xfb, 3, 1}}}, "truncated"},
		{"invalid command", []mptTestEntry{{"data", []byte{0x81, 1, 60, 0}}}, "unsupported"},
		{"invalid channel", []mptTestEntry{{"data", []byte{0x82, 0}}}, "channel"},
		{"trailing data", []mptTestEntry{{"data", []byte{0, 0}}}, "trailing"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pattern := mptTestBlock("mptP", tt.fields)
			block := mptTestBlock("mptPc", []mptTestEntry{{"num", []byte{1, 0}}, {"\x00\x00", pattern}})
			_, err := applyMPTPatternOverrides(block, []PatternAsset{{Rows: [][]ModCommand{{{}}}}}, 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %s", err, tt.want)
			}
		})
	}
}
