package mod

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"
)

// This inventories the target using existing parsers; it does not authorize playback.
func TestAuditTargetMPTStructure(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_TARGET")
	if path == "" {
		t.Skip("set MPTECS_MPTM_TARGET to the Triangel Island fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bounds, err := locateMPTExtensions(data)
	if err != nil {
		t.Fatal(err)
	}
	u16 := func(i int) int { return int(binary.LittleEndian.Uint16(data[i:])) }
	ptr := func(i int) int { return int(binary.LittleEndian.Uint32(data[i:])) }
	orders, instruments, samples, patterns := u16(32), u16(34), u16(36), u16(38)
	extensions, err := parseMPTExtensions(data[bounds.propertiesStart:bounds.serializedStart], instruments)
	if err != nil {
		t.Fatal(err)
	}
	sequences, err := parseMPTSequences(data[bounds.serializedStart:bounds.serializedEnd], patterns)
	if err != nil {
		t.Fatal(err)
	}
	for i, seq := range sequences.sequences {
		defaults, err := resolveMPTSequenceDefaults(seq, mptSequenceDefaults{tempo: uint32(data[51]) * 10000, speed: uint32(data[50])})
		if err != nil {
			t.Errorf("sequence %d defaults: %v", i, err)
		} else {
			t.Logf("sequence %d: %d orders, effective defaults %+v", i, len(seq.orders), defaults)
		}
	}
	instrumentTable := 192 + orders
	for i := range instruments {
		pos := ptr(instrumentTable + 4*i)
		if pos == 0 {
			t.Errorf("instrument %d: missing header", i+1)
			continue
		}
		inst, err := parseITInstrument(data[pos:pos+554], samples)
		if err != nil {
			t.Errorf("instrument %d: %v", i+1, err)
			continue
		}
		if inst.DuplicateCheck != 0 || inst.NewNoteAction != 0 {
			t.Logf("instrument %d: NNA %d, DCT %d", i+1, inst.NewNoteAction, inst.DuplicateCheck)
		}
		if i < len(extensions.instrumentFadeout) && extensions.instrumentFadeout[i] != uint32(inst.Fadeout)*32 {
			t.Logf("instrument %d: header fadeout scaled to MPT units %d, extended fadeout %d", i+1, uint32(inst.Fadeout)*32, extensions.instrumentFadeout[i])
		}
	}
	sampleTable := instrumentTable + 4*instruments
	encodings := make(map[string]int)
	for i := range samples {
		pos := ptr(sampleTable + 4*i)
		if pos == 0 {
			encodings["absent"]++
			continue
		}
		h := data[pos : pos+80]
		encodings[fmt.Sprintf("flags=%02x conversion=%02x", h[18], h[46])]++
		t.Logf("sample %d: flags=%02x conversion=%02x length=%d", i+1, h[18], h[46], binary.LittleEndian.Uint32(h[48:]))
	}
	patternTable := sampleTable + 4*samples
	effects, volumes := make(map[string]int), make(map[byte]int)
	instrumentOnly, commands := 0, 0
	for i := range patterns {
		pos := ptr(patternTable + 4*i)
		if pos == 0 {
			continue
		}
		size, rows := u16(pos), u16(pos+2)
		if rows < 1 || rows > 1024 {
			t.Errorf("pattern %d has invalid row count %d", i, rows)
			continue
		}
		pattern, err := unpackITPattern(data[pos+8:pos+8+size], rows)
		if err != nil {
			t.Errorf("pattern %d: %v", i, err)
			continue
		}
		for row, cells := range pattern.Rows {
			for channel, cmd := range cells {
				if cmd.Instrument != 0 && cmd.Note == 0 {
					instrumentOnly++
				}
				if int(cmd.Instrument) > instruments {
					t.Errorf("pattern %d row %d channel %d: missing instrument %d", i, row, channel+1, cmd.Instrument)
				}
				if cmd.HasVolume {
					volumes[cmd.Volume]++
				}
				if cmd.EffectCmd != 0 {
					effects[fmt.Sprintf("%02x:%02x", cmd.EffectCmd, cmd.EffectArg)]++
					commands++
				}
			}
		}
	}
	keys := make([]string, 0, len(effects))
	for key := range effects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.Logf("mapped effect %s: %d", key, effects[key])
	}
	t.Logf("sample encodings: %v", encodings)
	if encodings["flags=09 conversion=40"] != 8 || encodings["flags=19 conversion=40"] != 8 || encodings["flags=0f conversion=05"] != 2 || len(encodings) != 3 {
		t.Error("unexpected target sample encodings")
	}
	t.Logf("volume commands: %v", volumes)
	t.Logf("patterns: %d; effect cells: %d; instrument-only cells: %d", patterns, commands, instrumentOnly)
	if extensions.tempoMode == nil || extensions.mixLevels == nil {
		t.Error("target tempo mode or mix level is absent")
	} else {
		t.Logf("header: speed=%d tempo=%d rows/beat=%d flags=%04x; extensions: tempo mode=%d mix levels=%d", data[50], data[51], data[30], u16(44), *extensions.tempoMode, *extensions.mixLevels)
	}
	if instrumentOnly != 0 {
		t.Errorf("target uses %d instrument-only cells, which the IT loader rejects", instrumentOnly)
	}
	if _, err := loadIT(data); err == nil || err.Error() != "MPTM modules are not yet supported" {
		t.Errorf("MPTM playback gate returned %v", err)
	}
}
