package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITVolumeVibratoPortamento(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				for _, volume := range []byte{203, 204, 212} {
					for _, glissando := range []bool{false, true} {
						name := fmt.Sprintf("legacy=%t/linear=%t/old=%t/volume=%d/glissando=%t", legacy, linear, old, volume, glissando)
						t.Run(name, func(t *testing.T) {
							rows := []byte{0x81, 11, 48, 1, 8, 0x8f, 0}
							if glissando {
								rows = append(rows, 0x81, 8, 19, 0x11, 0)
							}
							rows = append(rows, 0x81, 13, 60, volume, 7, 0x5f, 0,
								0x81, 12, volume, 7, 0, 0, 0x81, 12, volume, 7, 0, 0)
							b := testITPitchFixture(rows, 6, linear, legacy)
							binary.LittleEndian.PutUint32(b[256+60:], 24000)
							if old {
								b[44] |= 0x10
							}
							ref, actual := referencePCMWithChunks(t, b, "2", 600, 97)
							baseRef, baseActual := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
							start := 6
							if glissando {
								start = 12
							}
							for tick := start; tick < start+18; tick++ {
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

func TestITVolumeVibratoPortamentoValidation(t *testing.T) {
	row := []byte{0x81, 12, 204, 7, 0x5f, 0}
	if _, err := unpackITPatternMode(row, 1, false); err != nil {
		t.Fatal(err)
	}
	row[2] = 203
	if _, err := unpackITPatternMode(row, 1, true); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
		t.Fatalf("MPTM combination: %v", err)
	}
	row[3] = 6
	if _, err := unpackITPatternMode(row, 1, false); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
		t.Fatalf("IT pitch slide combination: %v", err)
	}
}

func TestOpenMPTITVolumeVibratoPortamentoInstruments(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
	}{
		{"new target", []byte{0x81, 13, 60, 204, 7, 0x18, 0}},
		{"instrument only", []byte{0x81, 14, 1, 204, 7, 0, 0}},
		{"no note", []byte{0x81, 12, 204, 7, 0, 0}},
	} {
		for _, legacy := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/compatible=%t", tt.name, legacy, compatible), func(t *testing.T) {
					rows := []byte{0x81, 11, 48, 1, 8, 0x8f, 0, 0x81, 9, 60, 7, 0x18, 0}
					rows = append(rows, tt.row...)
					rows = append(rows, 0x81, 12, 204, 7, 0, 0)
					fixture := testITPitchFixture(rows, 6, true, legacy)
					b := testITInstrument()
					copy(b[40:44], fixture[40:44])
					b[44] |= 8
					if compatible {
						b[44] |= 0x20
					}
					binary.LittleEndian.PutUint32(b[256+60:], 24000)
					binary.LittleEndian.PutUint32(b[256+56:], 3840)
					copy(b[336:512], fixture[336:512])
					copy(b[512:512+8192], fixture[512:512+8192])
					b[len(b)-554+17], b[len(b)-554+304] = 0, 0
					ref, actual := referencePCMWithChunks(t, b, "2", 480, 97)
					_, whole := referencePCMWithChunks(t, b, "2", 480, 0)
					for i := range actual {
						if actual[i] != whole[i] {
							t.Fatalf("chunk-dependent PCM at sample %d", i)
						}
					}
					baseRef, baseActual := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
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
