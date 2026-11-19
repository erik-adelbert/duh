package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mptDuplicateNoteCutFixture(t *testing.T, version uint32, strict, realMapping, mappedCollision bool, nextInstrument byte, dct byte) []byte {
	t.Helper()
	b := mptDuplicateInstrumentOffFixture(t, 0, nextInstrument, false)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
	flag := bytesIndex(t, b, ".FSM") + 6
	b[flag+13] &^= 1
	if strict {
		b[flag+13] |= 1
	}
	b[flag+int(mptITRealNoteMapping)/8] &^= 1 << (uint(mptITRealNoteMapping) % 8)
	if realMapping {
		b[flag+int(mptITRealNoteMapping)/8] |= 1 << (uint(mptITRealNoteMapping) % 8)
	}
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	b[instrument+18], b[instrument+19] = dct, 0
	note := byte(60)
	if mappedCollision {
		note = 64
		b[instrument+64+2*64] = 60
	}
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	copy(b[pattern+8:], []byte{0x81, 3, 60, 2, 0, 0x81, 3, note, nextInstrument, 0})
	return b
}

func TestOpenMPTMDuplicateNoteCut(t *testing.T) {
	for _, profile := range []struct {
		version             uint32
		strict, realMapping bool
	}{
		{0x01280100, false, true},
		{0x01311400, false, true},
		{0x01311400, true, false},
		{0x01311400, true, true},
	} {
		for _, collision := range []bool{false, true} {
			for _, next := range []byte{1, 2} {
				t.Run(fmt.Sprintf("%08x/strict=%t/real=%t/mapped=%t/next=%d", profile.version, profile.strict, profile.realMapping, collision, next), func(t *testing.T) {
					fixture := func(dct byte) []byte {
						return mptDuplicateNoteCutFixture(t, profile.version, profile.strict, profile.realMapping, collision, next, dct)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(1), 140)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0), 140)
					for _, window := range [][2]int{{55, 65}, {90, 100}} {
						rbase, abase := rmsWindow(baseRef, window[0], window[1]), rmsWindow(baseActual, window[0], window[1])
						if rbase < .00001 || abase < .00001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, window[0], window[1])/rbase, rmsWindow(actual, window[0], window[1])/abase
						if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .09 {
							t.Fatalf("%d ms: OpenMPT ratio %.4f, tracker %.4f", window[0], want, got)
						}
						matched := next == 2 && (!collision || !profile.strict || !profile.realMapping || profile.version < 0x01290000)
						if matched && want > .8 || !matched && math.Abs(want-1) > .05 {
							t.Fatalf("reference does not demonstrate expected duplicate match=%t: %.4f", matched, want)
						}
					}
				})
			}
		}
	}
}

func TestMPTDuplicateNoteCutValidation(t *testing.T) {
	b := mptDuplicateNoteCutFixture(t, 0x01280100, false, true, false, 2, 1)
	if _, err := loadMPTM(b); err != nil {
		t.Fatal(err)
	}
	old := append([]byte(nil), b...)
	binary.LittleEndian.PutUint32(old[bytesIndex(t, old, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(old); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
		t.Fatalf("old profile: %v", err)
	}
	ptr := int(binary.LittleEndian.Uint32(b[198:]))
	b[ptr+60] = 129
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "plugin routing") {
		t.Fatalf("plugin: %v", err)
	}
}
