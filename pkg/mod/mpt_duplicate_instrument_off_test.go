package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mptDuplicateInstrumentOffFixture(t *testing.T, dct byte, nextInstrument byte, envelope bool) []byte {
	t.Helper()
	b := mptPitchPanProfileFixture(t, true)
	first := int(binary.LittleEndian.Uint32(b[198:]))
	second := int(binary.LittleEndian.Uint32(b[194:]))
	for _, ptr := range []int{first, second} {
		for i := range 120 {
			b[ptr+65+2*i] = 1
		}
		b[ptr+17], b[ptr+18], b[ptr+19] = 1, 0, 0
		binary.LittleEndian.PutUint16(b[ptr+20:], 128)
		if envelope {
			setTestEnvelope(b[ptr:ptr+554], 304, 5, []int8{64, 0}, []uint16{0, 8})
			b[ptr+308], b[ptr+309] = 0, 0
		} else {
			b[ptr+304] = 0
		}
	}
	b[first+18], b[first+19] = dct, 1
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	length := int(binary.LittleEndian.Uint16(b[pattern:]))
	clear(b[pattern+8 : pattern+8+length])
	copy(b[pattern+8:], []byte{
		0x81, 3, 60, 2, 0,
		0x81, 3, 64, nextInstrument, 0,
		0, 0, 0, 0,
	})
	return b
}

func TestMPTDuplicateInstrumentOffValidation(t *testing.T) {
	b := mptDuplicateInstrumentOffFixture(t, 3, 2, true)
	if _, err := loadMPTM(b); err != nil {
		t.Fatal(err)
	}
	old := append([]byte(nil), b...)
	binary.LittleEndian.PutUint32(old[bytesIndex(t, old, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(old); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
		t.Fatalf("old profile duplicate note-off: %v", err)
	}
	ptr := int(binary.LittleEndian.Uint32(b[198:]))
	b[ptr+65] = 2
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "OPL samples") {
		t.Fatalf("OPL duplicate note-off: %v", err)
	}
	b[ptr+65] = 1
	b[ptr+60] = 129
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "plugin routing") {
		t.Fatalf("plug-in duplicate note-off: %v", err)
	}
}

func TestOpenMPTMDuplicateInstrumentOff(t *testing.T) {
	for _, version := range []uint32{0x01260000, 0x01280100, 0x01311400} {
		for _, envelope := range []bool{false, true} {
			for _, next := range []byte{2, 1} {
				t.Run(fmt.Sprintf("version=%08x/envelope=%t/next=%d", version, envelope, next), func(t *testing.T) {
					fixture := func(dct byte) []byte {
						b := mptDuplicateInstrumentOffFixture(t, dct, next, envelope)
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(3), 180)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0), 180)
					for _, win := range [][2]int{{55, 65}, {90, 100}, {130, 140}} {
						rbase, abase := rmsWindow(baseRef, win[0], win[1]), rmsWindow(baseActual, win[0], win[1])
						if rbase < .00001 || abase < .00001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, win[0], win[1])/rbase, rmsWindow(actual, win[0], win[1])/abase
						if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || math.Abs(want-got) > .09 {
							t.Fatalf("%d ms: OpenMPT ratio %.4f, tracker %.4f", win[0], want, got)
						}
						if win[0] == 130 && (next == 2 && want > .9 || next == 1 && math.Abs(want-1) > .01) {
							t.Fatalf("comparison did not distinguish matching instruments: %.4f", want)
						}
					}
				})
			}
		}
	}
}
