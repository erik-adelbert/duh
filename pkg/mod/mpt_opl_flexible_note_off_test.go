package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestOpenMPTFlexibleOPLNoteOff(t *testing.T) {
	flags := make([]byte, 13)
	copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
	flags[12] = 8 // kOPLFlexibleNoteOff, bit 99
	for _, tt := range []struct {
		name string
		note byte
	}{
		{"cut", 254},
		{"release", 255},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{0x81, 1, tt.note, 0})
			instrument := int(binary.LittleEndian.Uint32(b[194:]))
			b[instrument+304] = 0
			sample := int(binary.LittleEndian.Uint32(b[206:]))
			patchOffset := int(binary.LittleEndian.Uint32(b[sample+72:]))
			patch := oplPatch{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x01, 0x01, 0, 0, 0, 0}
			copy(b[patchOffset:patchOffset+12], patch[:])
			loaded, err := loadITData(b, true)
			if err != nil {
				t.Fatal(err)
			}
			w := NewTracker(loaded.Assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			actual := make([]float32, 5760*2)
			if n := w.Read(actual, 5760); n != 5760 {
				t.Fatalf("short render %d", n)
			}
			ref := referenceTransition(t, b)
			for name, pcm := range map[string][]float32{"tracker": actual, "OpenMPT": ref} {
				before, after := rmsWindow(pcm, 45, 70), rmsWindow(pcm, 95, 115)
				if before < .001 {
					t.Fatalf("%s initial note silent: %g", name, before)
				}
				if tt.note == 254 && after/before > .02 {
					t.Fatalf("%s cut did not silence note: before %g, after %g", name, before, after)
				}
				if tt.note == 255 && after/before < .1 {
					t.Fatalf("%s release did not preserve the tail: before %g, after %g", name, before, after)
				}
			}
		})
	}
}

func TestOpenMPTOPLNoteFade(t *testing.T) {
	for _, flexible := range []bool{false, true} {
		name := "legacy"
		if flexible {
			name = "flexible"
		}
		t.Run(name, func(t *testing.T) {
			flags := make([]byte, 13)
			copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
			if flexible {
				flags[12] = 8
			}
			b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{0x81, 1, 253, 0})
			instrument := int(binary.LittleEndian.Uint32(b[194:]))
			b[instrument+304] = 0
			binary.LittleEndian.PutUint16(b[instrument+20:], 0)
			sample := int(binary.LittleEndian.Uint32(b[206:]))
			patchOffset := int(binary.LittleEndian.Uint32(b[sample+72:]))
			copy(b[patchOffset:patchOffset+12], []byte{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0})
			loaded, err := loadITData(b, true)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITCompatibility.oplFlexibleNoteOff = flexible
			w := NewTracker(loaded.Assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			actual := make([]float32, 5760*2)
			if n := w.Read(actual, 5760); n != 5760 {
				t.Fatalf("short render %d", n)
			}
			ref := referenceTransition(t, b)
			for label, pcm := range map[string][]float32{"tracker": actual, "OpenMPT": ref} {
				before, after := rmsWindow(pcm, 45, 70), rmsWindow(pcm, 95, 115)
				if before < .001 {
					t.Fatalf("%s initial note silent: %g", label, before)
				}
				if flexible && after/before < .1 || !flexible && after/before > .02 {
					t.Fatalf("%s fade ratio %g, flexible %v", label, after/before, flexible)
				}
			}
		})
	}
}

func TestMPTNoteFadeParsing(t *testing.T) {
	data := []byte{0x81, 1, 253, 0, 0x81, 0x10, 0}
	if _, err := unpackITPattern(data, 2); err == nil {
		t.Fatal("ordinary IT unexpectedly accepted MPTM note fade")
	}

	pattern, err := unpackITPatternMode(data, 2, true)
	if err != nil || pattern.Rows[0][0].Note != 253 || pattern.Rows[1][0].Note != 253 {
		t.Fatalf("MPTM note fade or reuse: %+v, error %v", pattern, err)
	}
	b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0x81, 3, 253, 1, 0})
	if _, err := loadITData(b, true); err == nil || !strings.Contains(err.Error(), "note fade with other commands") {
		t.Fatalf("expected rejection of combined note fade, got %v", err)
	}
}
