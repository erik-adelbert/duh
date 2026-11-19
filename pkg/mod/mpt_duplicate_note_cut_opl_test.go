package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMDuplicateNoteCutOPL(t *testing.T) {
	for _, profile := range []struct {
		version             uint32
		strict, realMapping bool
		nna                 byte
	}{
		{0x01280100, false, true, 1},
		{0x01280100, false, true, 2},
		{0x01311400, false, true, 1},
		{0x01311400, true, true, 1},
	} {
		for _, collision := range []bool{false, true} {
			for _, next := range []byte{1, 2} {
				t.Run(fmt.Sprintf("%08x/strict=%t/nna=%d/mapped=%t/next=%d", profile.version, profile.strict, profile.nna, collision, next), func(t *testing.T) {
					fixture := func(dct byte) []byte {
						b := mptDuplicateNoteCutFixture(t, profile.version, profile.strict, profile.realMapping, collision, next, dct)
						for i := range 2 {
							ptr := int(binary.LittleEndian.Uint32(b[194+i*4:]))
							b[ptr+17] = profile.nna
							for note := range 120 {
								b[ptr+65+note*2] = 2
							}
						}
						sample := int(binary.LittleEndian.Uint32(b[206:]))
						patch := int(binary.LittleEndian.Uint32(b[sample+72:]))
						copy(b[patch:patch+12], []byte{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x05, 0x05, 0, 0, 0, 0})
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(1), 140)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(0), 140)
					for _, window := range [][2]int{{55, 65}, {90, 100}} {
						rbase, abase := rmsWindow(baseRef, window[0], window[1]), rmsWindow(baseActual, window[0], window[1])
						if rbase < .00001 || abase < .00001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, window[0], window[1])/rbase, rmsWindow(actual, window[0], window[1])/abase
						if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .15 {
							t.Fatalf("%d ms: OpenMPT ratio %.4f, tracker %.4f", window[0], want, got)
						}
						matched := profile.nna == 1 && next == 2 && (!collision || !profile.strict)
						if matched && want > .8 || !matched && math.Abs(want-1) > .05 {
							t.Fatalf("reference does not demonstrate expected duplicate match=%t: %.4f", matched, want)
						}
					}
				})
			}
		}
	}
}
