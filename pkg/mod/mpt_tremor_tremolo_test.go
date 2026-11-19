package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMTremorTremolo(t *testing.T) {
	tests := []struct {
		name   string
		rows   []byte
		tremor bool
	}{
		{"I34 memory", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0x81, 8, 9, 0, 0}, true},
		{"I11", []byte{0x81, 11, 60, 1, 9, 0x11, 0, 0x81, 8, 9, 0, 0}, true},
		{"I03", []byte{0x81, 11, 60, 1, 9, 0x03, 0, 0x81, 8, 9, 0, 0}, true},
		{"I30", []byte{0x81, 11, 60, 1, 9, 0x30, 0, 0x81, 8, 9, 0, 0}, true},
		{"I interrupted", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0, 0x81, 8, 9, 0, 0}, true},
		{"I new note", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0x81, 11, 60, 1, 9, 0, 0}, true},
		{"I before note", []byte{0x81, 8, 9, 0x34, 0, 0x81, 11, 60, 1, 9, 0, 0}, true},
		{"I11 after memory", []byte{0x81, 11, 60, 1, 9, 0x34, 0, 0x81, 8, 9, 0x11, 0}, true},
		{"R memory", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0x81, 8, 18, 0, 0}, false},
		{"R partial memory", []byte{0x81, 15, 60, 1, 32, 18, 0x45, 0, 0x81, 8, 18, 0x80, 0, 0x81, 8, 18, 0x03, 0}, false},
		{"R interrupted", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0, 0x81, 8, 18, 0, 0}, false},
		{"R new note", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0x81, 15, 60, 1, 32, 18, 0, 0}, false},
		{"R zero volume", []byte{0x81, 15, 60, 1, 0, 18, 0x4f, 0, 0x81, 8, 18, 0, 0}, false},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, tremor := range []bool{false, true} {
			for _, wave := range []bool{false, true} {
				for _, old := range []bool{false, true} {
					for _, version := range []uint32{0x01280100, 0x01321200} {
						for _, retrigger := range []bool{false, true} {
							name := fmt.Sprintf("%s/tremor=%t/wave=%t/old=%t/version=%08x/retrig=%t", tt.name, tremor, wave, old, version, retrigger)
							t.Run(name, func(t *testing.T) {
								flags := all &^ (1<<mptITTremor | 1<<mptITVibratoTremoloPanbrello)
								if tremor {
									flags |= 1 << mptITTremor
								}
								if wave {
									flags |= 1 << mptITVibratoTremoloPanbrello
								}
								if !retrigger {
									flags &^= 1 << mptITRetrigger
								}
								fixture := func(rows []byte) []byte {
									b := mptSharedEffectFixture(t, rows, flags)
									b[50] = 6
									if old {
										b[44] |= 0x10
									}
									binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
									return b
								}
								ref, actual := mptSharedEffectRenderForDuration(t, fixture(tt.rows), 360)
								if tt.tremor {
									for tick := range 18 {
										ms := tick*20 + 15
										want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
										if math.IsNaN(want) || math.IsNaN(got) || math.IsInf(want, 0) || math.IsInf(got, 0) || (want > .001) != (got > .001) {
											t.Errorf("tick %d: OpenMPT RMS %.4f tracker %.4f", tick, want, got)
										}
									}
								} else {
									baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture([]byte{0x81, 7, 60, 1, 32, 0}), 360)
									for tick := range 18 {
										assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .05)
									}
								}
							})
						}
					}
				}
			}
		}
	}
}
