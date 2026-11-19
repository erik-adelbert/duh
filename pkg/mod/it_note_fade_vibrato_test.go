package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITNoteFadeVibrato(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				for _, arg := range []byte{0, 0x92, 0x8f} {
					t.Run(fmt.Sprintf("legacy=%t/linear=%t/old=%t/H%02X", legacy, linear, old, arg), func(t *testing.T) {
						fixture := func(fadeout uint16) []byte {
							b := testITInstrument()
							b[50] = 6
							if linear {
								b[44] |= 8
							}
							if old {
								b[44] |= 0x10
							}
							if legacy {
								binary.LittleEndian.PutUint16(b[40:], 0x217)
								binary.LittleEndian.PutUint16(b[42:], 0x200)
							}
							h := b[len(b)-554:]
							h[17], h[304] = 0, 0
							binary.LittleEndian.PutUint16(h[20:], fadeout)
							sample := testITPitchFixture(nil, 6, linear, legacy)
							copy(b[512:512+8192], sample[512:512+8192])
							binary.LittleEndian.PutUint32(b[256+60:], 24000)
							binary.LittleEndian.PutUint32(b[256+56:], 3840)
							rows := append([]byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 9, 246, 8, arg, 0}, make([]byte, 62)...)
							binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
							copy(b[344:], rows)
							return b
						}
						ref, actual := referencePCMWithChunks(t, fixture(32), "2", 360, 97)
						refBase, actualBase := referencePCMWithChunks(t, fixture(0), "2", 360, 97)
						minPitch, maxPitch := math.Inf(1), 0.0
						for tick := 6; tick < 16; tick++ {
							wantPitch := referenceTickFrequency(t, ref, tick)
							gotPitch := referenceTickFrequency(t, actual, tick)
							if tick < 12 {
								minPitch, maxPitch = math.Min(minPitch, wantPitch), math.Max(maxPitch, wantPitch)
							}
							if math.IsNaN(gotPitch) || math.Abs(gotPitch/wantPitch-1) > .003 {
								t.Errorf("tick %d pitch: OpenMPT %.3f, tracker %.3f", tick, wantPitch, gotPitch)
							}
							ms := tick*20 + 15
							rb, ab := rmsWindow(refBase, ms, ms+3), rmsWindow(actualBase, ms, ms+3)
							if rb < .001 || ab < .001 {
								t.Fatal("silent comparison baseline")
							}
							want, got := rmsWindow(ref, ms, ms+3)/rb, rmsWindow(actual, ms, ms+3)/ab
							if math.IsNaN(got) || math.Abs(got-want) > .05 {
								t.Errorf("tick %d fade: OpenMPT %.3f, tracker %.3f", tick, want, got)
							}
							if tick == 15 && want > .95 {
								t.Error("reference note fade did not measurably reduce volume")
							}
						}
						if maxPitch/minPitch < 1.005 {
							t.Error("reference vibrato did not measurably vary pitch")
						}
					})
				}
			}
		}
	}
}

func TestITNoteFadeVibratoValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"H92", []byte{0x81, 9, 246, 8, 0x92, 0}, true},
		{"H00", []byte{0x81, 9, 246, 8, 0, 0}, true},
		{"instrument", []byte{0x81, 11, 246, 1, 8, 0x92, 0}, false},
		{"volume", []byte{0x81, 13, 246, 32, 8, 0x92, 0}, false},
		{"fine vibrato", []byte{0x81, 9, 246, 21, 0x92, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	b := mptS70Fixture(t, []byte{0x81, 9, 253, 8, 0x92, 0})
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM fade with vibrato: %v", err)
	}
}
