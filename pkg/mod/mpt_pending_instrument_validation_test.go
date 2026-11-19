package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mptRemainingFlagsFixture(t *testing.T, rows []byte, emptyMap, bit123, bit134 bool) []byte {
	t.Helper()
	b := mptOffsetInstrumentFixture(t, rows, false, false)
	if emptyMap {
		instrument := int(binary.LittleEndian.Uint32(b[198:]))
		b[instrument+65+64*2] = 0
	}
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	flags := make([]byte, 17)
	copy(flags, b[field+6:field+6+length])
	if bit123 {
		flags[mptITEmptyNoteMapIgnoreCellBit/8] |= 1 << (mptITEmptyNoteMapIgnoreCellBit % 8)
	}
	if bit134 {
		flags[mptITNoteCutWithPortaBit/8] |= 1 << (mptITNoteCutWithPortaBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	tail := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(tail))
}

func TestOpenMPTEmptyNoteMapIgnoredCell(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
	}{
		{"global volume", []byte{0x81, 11, 64, 2, 22, 32, 0}},
		{"tempo", []byte{0x81, 11, 64, 2, 20, 200, 0}},
		{"speed", []byte{0x81, 11, 64, 2, 1, 6, 0}},
		{"local volume", []byte{0x81, 7, 64, 2, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := append([]byte{0x81, 3, 60, 1, 0}, tt.row...)
			rows = append(rows, 0x81, 3, 68, 1, 0)
			b := mptRemainingFlagsFixture(t, rows, true, true, false)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 180)
			assertMPTPCMCorrelation(t, ref, actual, 90, 130)
			assertMPTPCMCorrelation(t, ref, actual, 140, 170)
			base := mptRemainingFlagsFixture(t, []byte{0x81, 3, 60, 1, 0, 0, 0x81, 3, 68, 1, 0}, true, true, false)
			baseRef, baseActual := mptSharedEffectRenderForDuration(t, base, 180)
			for _, ms := range []int{90, 150} {
				want := channelRMS(ref, ms, ms+5, 0) / channelRMS(baseRef, ms, ms+5, 0)
				got := channelRMS(actual, ms, ms+5, 0) / channelRMS(baseActual, ms, ms+5, 0)
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .03 {
					t.Errorf("%dms: OpenMPT ratio %g, tracker %g", ms, want, got)
				}
			}
		})
	}
}

func TestMPTPendingInstrumentAndNoteCutValidation(t *testing.T) {
	tests := []struct {
		name           string
		rows           []byte
		emptyMap       bool
		bit123, bit134 bool
		expected       string
	}{
		{"inert flags", []byte{0x81, 3, 60, 1, 0, 0x81, 1, 64, 0}, false, true, true, ""},
		{"instrument only", []byte{0x81, 2, 1, 0}, false, true, false, "pending instrument"},
		{"instrument with special note", []byte{0x81, 3, 255, 1, 0}, false, true, false, "pending instrument"},
		{"instrument delay", []byte{0x81, 11, 60, 1, 19, 0xd1, 0}, false, true, false, "pending instrument"},
		{"instrument S00", []byte{0x81, 11, 60, 1, 19, 0, 0}, false, true, false, "pending instrument"},
		{"empty map then implicit note", []byte{0x81, 3, 64, 2, 0, 0x81, 1, 60, 0}, true, true, false, "instrumentless note"},
		{"note cut", []byte{0x81, 11, 60, 1, 19, 0xc1, 0}, false, false, true, ""},
		{"note cut then instrument", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 2, 1, 0}, false, false, true, "note cut"},
		{"note cut then retrigger", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 8, 17, 3, 0}, false, false, true, "note cut"},
		{"note cut flag disabled", []byte{0x81, 11, 60, 1, 19, 0xc1, 0}, false, false, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := mptRemainingFlagsFixture(t, tt.rows, tt.emptyMap, tt.bit123, tt.bit134)
			_, err := loadMPTM(b)
			if tt.expected == "" && err != nil {
				t.Fatal(err)
			}
			if tt.expected != "" && (err == nil || !strings.Contains(err.Error(), tt.expected)) {
				t.Fatalf("expected %q, got %v", tt.expected, err)
			}
		})
	}
}

func TestMPTEmptyMapChunking(t *testing.T) {
	b := mptRemainingFlagsFixture(t, []byte{0x81, 3, 60, 1, 0, 0x81, 11, 64, 2, 22, 32, 0, 0x81, 3, 68, 1, 0}, true, true, true)
	assertMPTPatternDelayChunking(t, b, 9000)
}

func TestOpenMPTEmptyMapFlagChangesPlayback(t *testing.T) {
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 11, 64, 2, 22, 32, 0}
	enabled := referenceMPTMForDuration(t, mptRemainingFlagsFixture(t, rows, true, true, false), 120)
	disabled := referenceMPTMForDuration(t, mptRemainingFlagsFixture(t, rows, true, false, false), 120)
	if on, off := channelRMS(enabled, 100, 110, 0), channelRMS(disabled, 100, 110, 0); on < .001 || off > on*.5 {
		t.Fatalf("reference does not distinguish flag: enabled %g, disabled %g", on, off)
	}
}

func TestMPTRemainingFlagsVersionProjection(t *testing.T) {
	for _, tt := range []struct {
		bit     int
		version uint32
	}{
		{123, 0x01320013},
		{134, 0x01320102},
	} {
		for _, version := range []uint32{tt.version - 1, tt.version, tt.version & 0xffff0000} {
			t.Run(fmt.Sprintf("bit=%d/version=%x", tt.bit, version), func(t *testing.T) {
				rows := []byte{0x81, 2, 1, 0}
				if tt.bit == 134 {
					rows = []byte{0x81, 8, 19, 0xc1, 0, 0x81, 2, 1, 0}
				}

				b := mptRemainingFlagsFixture(t, rows, false, tt.bit == 123, tt.bit == 134)
				binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
				_, err := loadMPTM(b)
				active := mptBehaviourAvailable(version, tt.version)
				if (err != nil) != active {
					t.Fatalf("active=%v error=%v", active, err)
				}
			})
		}
	}
}
