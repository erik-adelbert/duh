package mod

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestMPTPatternOverridesTarget(t *testing.T) {
	path := os.Getenv("MPTECS_PATTERN_TARGET")
	if path == "" {
		t.Skip("set MPTECS_PATTERN_TARGET to a module with serialized patterns")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseMPTSerializedBlock(data[bounds.serializedStart:bounds.serializedEnd], "mptm")
	if err != nil {
		t.Fatal(err)
	}
	count := int(binary.LittleEndian.Uint16(data[38:]))
	offset := 192 + int(binary.LittleEndian.Uint16(data[32:])) +
		4*int(binary.LittleEndian.Uint16(data[34:])) + 4*int(binary.LittleEndian.Uint16(data[36:]))
	patterns := make([]PatternAsset, count)
	for i := range patterns {
		ptr := int(binary.LittleEndian.Uint32(data[offset+i*4:]))
		rows := 64
		if ptr != 0 {
			rows = int(binary.LittleEndian.Uint16(data[ptr+2:]))
		}
		patterns[i].Rows = make([][]ModCommand, rows)
		for row := range patterns[i].Rows {
			patterns[i].Rows[row] = make([]ModCommand, MaxTracks)
		}
	}
	ext, err := parseMPTExtensions(data[bounds.propertiesStart:bounds.serializedStart], int(binary.LittleEndian.Uint16(data[34:])))
	if err != nil || ext.channels == nil {
		t.Fatalf("extensions: %v", err)
	}
	bypassed, err := applyMPTPatternOverrides(root["mptPc"], patterns, int(*ext.channels))
	if err != nil || !bypassed {
		t.Fatalf("bypassed %t, error %v", bypassed, err)
	}
}
