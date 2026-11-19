package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITVolumeVibratoWithS77(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, old := range []bool{false, true} {
			for _, volume := range []byte{203, 204, 212} {
				for _, newNote := range []bool{false, true} {
					t.Run(fmt.Sprintf("legacy=%t/old=%t/volume=%d/note=%t", legacy, old, volume, newNote), func(t *testing.T) {
						fixture := func(control byte) []byte {
							b := testITInstrument()
							b[50], b[44] = 6, b[44]|8
							if old {
								b[44] |= 0x10
							}
							if legacy {
								binary.LittleEndian.PutUint16(b[40:], 0x217)
								binary.LittleEndian.PutUint16(b[42:], 0x200)
							}
							h := b[len(b)-554:]
							h[17] = 0
							setTestEnvelope(h, 304, 1, []int8{64, 16}, []uint16{0, 24})
							pitch := testITPitchFixture(nil, 6, true, legacy)
							copy(b[512:512+8192], pitch[512:512+8192])
							rows := []byte{0x81, 11, 60, 1, 8, 0x8f, 0}
							if newNote {
								rows = append(rows, 0x81, 15, 60, 1, volume, 19, control, 0)
							} else {
								rows = append(rows, 0x81, 12, volume, 19, control, 0)
							}
							rows = append(rows, make([]byte, 62)...)
							binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
							copy(b[344:], rows)
							return b
						}
						ref, actual := referencePCMWithChunks(t, fixture(0x77), "2", 360, 97)
						for tick := range 18 {
							want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
							if math.IsNaN(got) || math.Abs(got/want-1) > .003 {
								t.Errorf("tick %d pitch: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
							}
							ms := tick*20 + 10
							wantVolume := channelRMS(ref, ms, ms+9, 0) / channelRMS(ref, 10, 19, 0)
							gotVolume := channelRMS(actual, ms, ms+9, 0) / channelRMS(actual, 10, 19, 0)
							if math.IsNaN(gotVolume) || math.Abs(gotVolume-wantVolume) > .12 {
								t.Errorf("tick %d volume: OpenMPT %.3f, tracker %.3f", tick, wantVolume, gotVolume)
							}
						}
						refEnabled, _ := referencePCMWithChunks(t, fixture(0x30), "2", 360, 97)
						disabled := channelRMS(ref, 330, 339, 0) / channelRMS(ref, 10, 19, 0)
						enabled := channelRMS(refEnabled, 330, 339, 0) / channelRMS(refEnabled, 10, 19, 0)
						if math.Abs(disabled-enabled) < .1 {
							t.Fatalf("reference envelope control had no measurable effect: disabled %.3f enabled %.3f", disabled, enabled)
						}
					})
				}
			}
		}
	}
}

func TestITVolumeVibratoS77Validation(t *testing.T) {
	for _, volume := range []byte{203, 204, 212} {
		row := []byte{0x81, 12, volume, 19, 0x77, 0}
		if _, err := unpackITPatternMode(row, 1, false); err != nil {
			t.Errorf("IT volume %d with S77: %v", volume, err)
		}
		if _, err := unpackITPatternMode(row, 1, true); err == nil {
			t.Errorf("MPTM volume %d with S77: got %v", volume, err)
		}
		row[4] = 0x78
		if _, err := unpackITPatternMode(row, 1, false); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
			t.Errorf("IT volume %d with S78: got %v", volume, err)
		}
	}
}
