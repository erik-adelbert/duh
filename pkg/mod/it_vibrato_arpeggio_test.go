package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITVolumeVibratoArpeggio(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				for _, volume := range []byte{203, 204, 212} {
					for _, arp := range []byte{0, 0x7b, 0xff} {
						name := fmt.Sprintf("legacy=%t/linear=%t/old=%t/vol=%d/J%02X", legacy, linear, old, volume, arp)
						t.Run(name, func(t *testing.T) {
							rows := []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 8, 10, 0x47, 0,
								0x81, 12, volume, 10, arp, 0, 0x81, 12, volume, 10, 0, 0}
							b := testITPitchFixture(rows, 6, linear, legacy)
							binary.LittleEndian.PutUint32(b[256+60:], 24000)
							if old {
								b[44] |= 0x10
							}
							ref, actual := referencePCMWithChunks(t, b, "2", 480, 97)
							baseRef, baseActual := referenceTickFrequency(t, ref, 6), referenceTickFrequency(t, actual, 6)
							for tick := 12; tick < 24; tick++ {
								want := referenceTickFrequency(t, ref, tick) / baseRef
								got := referenceTickFrequency(t, actual, tick) / baseActual
								if math.IsNaN(got) || math.Abs(got-want) > .003 {
									t.Errorf("tick %d: OpenMPT %.5f, tracker %.5f", tick, want, got)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestITVolumeVibratoArpeggioValidation(t *testing.T) {
	row := []byte{0x81, 12, 204, 10, 0x7b, 0}
	if _, err := unpackITPatternMode(row, 1, false); err != nil {
		t.Fatal(err)
	}
	row[2] = 203
	if _, err := unpackITPatternMode(row, 1, true); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
		t.Fatalf("MPTM combination: %v", err)
	}
}

func TestOpenMPTITVolumeVibratoArpeggioInstruments(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
	}{
		{"new note", []byte{0x81, 15, 60, 1, 204, 10, 0x7b, 0}},
		{"instrument only", []byte{0x81, 14, 1, 204, 10, 0x7b, 0}},
		{"no note", []byte{0x81, 12, 204, 10, 0x7b, 0}},
	} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				rows := append([]byte{0x81, 11, 60, 1, 8, 0x8f, 0}, tt.row...)
				pitchFixture := testITPitchFixture(rows, 6, true, legacy)
				b := testITInstrument()
				copy(b[40:44], pitchFixture[40:44])
				b[44] |= 8
				binary.LittleEndian.PutUint32(b[256+60:], 24000)
				binary.LittleEndian.PutUint32(b[256+56:], 3840)
				copy(b[336:512], pitchFixture[336:512])
				copy(b[512:512+8192], pitchFixture[512:512+8192])
				b[len(b)-554+304] = 0
				b[len(b)-554+17] = 0
				ref, actual := referencePCMWithChunks(t, b, "2", 240, 97)
				baseRef, baseActual := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
				for tick := 6; tick < 12; tick++ {
					want := referenceTickFrequency(t, ref, tick) / baseRef
					got := referenceTickFrequency(t, actual, tick) / baseActual
					if math.IsNaN(got) || math.Abs(got-want) > .003 {
						t.Errorf("tick %d: OpenMPT %.5f, tracker %.5f", tick, want, got)
					}
				}
			})
		}
	}
}
