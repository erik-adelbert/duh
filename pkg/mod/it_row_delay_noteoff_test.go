package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITRowDelayNoteOff(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, sameChannel := range []bool{false, true} {
			for _, delay := range []byte{0xe0, 0xe2, 0xef} {
				name := fmt.Sprintf("legacy=%t/same=%t/S%02X", legacy, sameChannel, delay)
				t.Run(name, func(t *testing.T) {
					b := testITInstrument()
					b[50], b[44] = 2, b[44]|8
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					copy(b[512:512+8192], testITPitchFixture(nil, 2, true, legacy)[512:512+8192])
					h := b[len(b)-554:]
					setTestEnvelope(h, 304, 5, []int8{64, 64, 0}, []uint16{0, 1, 5})
					h[308], h[309] = 1, 1
					rows := []byte{0x81, 3, 60, 1, 0}
					if sameChannel {
						rows = append(rows, 0x81, 9, 255, 19, delay, 0)
					} else {
						rows = append(rows, 0x81, 1, 255, 0x82, 8, 19, delay, 0)
					}
					rows = append(rows, 0x81, 3, 60, 1, 0)
					rows = append(rows, make([]byte, 61)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
					copy(b[344:], rows)
					onset := 40 * (2 + int(delay&15))
					ref, actual := referencePCMWithChunks(t, b, "2", onset+50, 97)
					refBase, actualBase := rmsWindow(ref, 10, 30), rmsWindow(actual, 10, 30)
					if refBase < .005 || actualBase < .005 {
						t.Fatal("fixture is silent before note-off")
					}
					for ms := 10; ms < onset+40; ms += 10 {
						if ms == 40 || ms == onset {
							continue
						}
						want, got := rmsWindow(ref, ms, ms+5)/refBase, rmsWindow(actual, ms, ms+5)/actualBase
						if math.IsNaN(got) || math.Abs(got-want) > .12 {
							t.Errorf("%dms: OpenMPT RMS %.5f, tracker %.5f", ms, want, got)
						}
					}
					for _, renderer := range []struct {
						name string
						pcm  []float32
					}{{"OpenMPT", ref}, {"tracker", actual}} {
						if rmsWindow(renderer.pcm, onset+10, onset+30) < .005 {
							t.Fatalf("%s: fixture is silent after the delayed row", renderer.name)
						}
						if delay&15 >= 2 && rmsWindow(renderer.pcm, 130, 150) > .001 {
							t.Fatalf("%s: note-off did not release before the delayed row ended", renderer.name)
						}
					}
				})
			}
		}
	}
}

func TestITRowDelayNoteOffValidation(t *testing.T) {
	for _, row := range [][]byte{
		{0x81, 11, 255, 1, 19, 0xe2, 0},
		{0x81, 9, 254, 19, 0xe2, 0},
	} {
		if _, err := loadIT(testITInstrumentWithRows(row)); err == nil || !strings.Contains(err.Error(), "pattern delay") {
			t.Errorf("unsupported delayed row %v: %v", row, err)
		}
	}
	if _, err := loadIT(testITInstrumentWithRows([]byte{0x81, 9, 246, 19, 0xe2, 0})); err == nil {
		t.Error("delayed note fade was accepted")
	}
	if _, err := loadIT(testITPitchFixture([]byte{0x81, 9, 255, 19, 0xe2, 0}, 2, true, false)); err == nil || !strings.Contains(err.Error(), "pattern delay") {
		t.Errorf("sample-mode note-off: %v", err)
	}
}

func TestOpenMPTITRowDelayNoteOffWithoutEnvelope(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 3, 60, 1, 0, 0x81, 9, 255, 19, 0xe2, 0, 0x81, 3, 60, 1, 0})
	b[50], b[len(b)-554+304] = 2, 0
	ref, actual := referencePCMWithChunks(t, b, "2", 200, 97)
	refBase, actualBase := rmsWindow(ref, 10, 30), rmsWindow(actual, 10, 30)
	if refBase < .005 || actualBase < .005 {
		t.Fatal("silent baseline")
	}
	for ms := 50; ms < 150; ms += 10 {
		want, got := rmsWindow(ref, ms, ms+5)/refBase, rmsWindow(actual, ms, ms+5)/actualBase
		if math.IsNaN(got) || math.Abs(got-want) > .12 {
			t.Errorf("%dms: OpenMPT %.3f, tracker %.3f", ms, want, got)
		}
	}
}

func testITInstrumentWithRows(rows []byte) []byte {
	b := testITInstrument()
	rows = append(append([]byte(nil), rows...), make([]byte, 64)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
	copy(b[344:], rows)
	return b
}
