package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestITVolume113OpenMPT(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"existing note", []byte{0x81, 3, 60, 1, 0, 0x81, 4, 113, 0}},
		{"note and volume slide", []byte{0x81, 15, 60, 1, 113, 4, 1, 0, 0x81, 12, 113, 4, 1, 0}},
		{"note and vibrato", []byte{0x81, 15, 60, 1, 113, 8, 0x15, 0, 0x81, 12, 113, 8, 0x15, 0}},
		{"down 4 and arpeggio", []byte{0x81, 15, 60, 1, 109, 10, 0x37, 0, 0x81, 12, 109, 10, 0x46, 0}},
	}
	for _, tt := range tests {
		for _, linear := range []bool{false, true} {
			for _, legacy := range []bool{false, true} {
				for _, instrument := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/linear=%t/legacy=%t/instrument=%t", tt.name, linear, legacy, instrument), func(t *testing.T) {
						b := testITPitchFixture(tt.rows, 6, linear, legacy)
						if instrument {
							withInstrument := testITInstrument()
							copy(withInstrument[512:512+8192], b[512:512+8192])
							withInstrument[44] = b[44] | 4
							copy(withInstrument[40:44], b[40:44])
							packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
							binary.LittleEndian.PutUint16(withInstrument[336:], uint16(len(packed)))
							copy(withInstrument[344:], packed)
							withInstrument[len(withInstrument)-554+304] = 0
							b = withInstrument
						}
						ref, actual := referencePCMWithChunks(t, b, "2", 240, 256)
						refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
						for tick := 1; tick < 12; tick++ {
							want := referenceTickFrequency(t, ref, tick) / refBase
							got := referenceTickFrequency(t, actual, tick) / actualBase
							if math.Abs(got-want) > .01 {
								t.Errorf("tick %d pitch ratio: OpenMPT %.5f tracker %.5f", tick, want, got)
							}
						}
					})
				}
			}
		}
	}
}

func TestITVolume113Validation(t *testing.T) {
	if _, err := unpackITPattern([]byte{0x81, 4, 113, 0}, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackITPattern([]byte{0x81, 4, 109, 0}, 1); err != nil {
		t.Fatal(err)
	}
	for _, effect := range []byte{5, 6, 7, 19} {
		arg := byte(1)
		if effect == 19 {
			arg = 0xd1
		}
		if _, err := unpackITPattern([]byte{0x81, 12, 113, effect, arg, 0}, 1); err == nil || !strings.Contains(err.Error(), "volume pitch slide") {
			t.Fatalf("effect %d: expected combination rejection, got %v", effect, err)
		}
	}
}
