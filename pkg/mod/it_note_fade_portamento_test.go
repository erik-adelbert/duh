package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeTonePortamento(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				for _, old := range []bool{false, true} {
					for _, arg := range []byte{0, 0x04, 0x18} {
						name := fmt.Sprintf("legacy=%t/linear=%t/compatible=%t/old=%t/G%02X", legacy, linear, compatible, old, arg)
						t.Run(name, func(t *testing.T) {
							fixture := func(fadeout uint16) []byte {
								rows := []byte{0x81, 3, 48, 1, 0, 0x81, 9, 60, 7, 0x08, 0,
									0x81, 9, 246, 7, arg, 0, 0x81, 8, 7, 0, 0}
								b := testITInstrumentWithRows(rows)
								b[50] = 3
								if linear {
									b[44] |= 8
								}
								if compatible {
									b[44] |= 0x20
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
								copy(b[512:512+8192], testITPitchFixture(nil, 3, linear, legacy)[512:512+8192])
								binary.LittleEndian.PutUint32(b[256+60:], 24000)
								binary.LittleEndian.PutUint32(b[256+56:], 3840)
								return b
							}
							ref, actual := referencePCMWithChunks(t, fixture(32), "2", 240, 97)
							refBase, actualBase := referencePCMWithChunks(t, fixture(0), "2", 240, 97)
							_, whole := referencePCMWithChunks(t, fixture(32), "2", 240, 0)
							for i := range actual {
								if actual[i] != whole[i] {
									t.Fatalf("chunk-dependent PCM at sample %d", i)
								}
							}
							for tick := 6; tick < 12; tick++ {
								wantPitch, gotPitch := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
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
								if tick == 11 && want > .95 {
									t.Error("reference note fade did not measurably reduce volume")
								}
							}
							if first, last := referenceTickFrequency(t, ref, 6), referenceTickFrequency(t, ref, 11); last/first < 1.01 {
								t.Errorf("reference portamento did not continue: %.2f to %.2f Hz", first, last)
							}
						})
					}
				}
			}
		}
	}
}

func TestITFadeTonePortamentoValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"G00", []byte{0x81, 9, 246, 7, 0, 0}, true},
		{"G18", []byte{0x81, 9, 246, 7, 0x18, 0}, true},
		{"instrument", []byte{0x81, 11, 246, 1, 7, 0, 0}, false},
		{"volume", []byte{0x81, 13, 246, 32, 7, 0, 0}, false},
		{"Lxx", []byte{0x81, 9, 246, 12, 1, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 9, 253, 7, 0, 0})); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}

func TestITFadeTonePortamentoVoiceIsolation(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 9, 246, 7, 0, 0})
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 32)
	w.PendingCmds[0] = ModCommand{Note: 49, Instrument: 1}
	VoiceAllocationSystem(w)
	w.PendingCmds[0] = ModCommand{Note: 50, Instrument: 1}
	VoiceAllocationSystem(w)
	past, current := VoiceID(0), w.TrackCursors[0].ActiveVoice
	if past == current || !w.VoiceLives[past].ITPastNote {
		t.Fatal("fixture did not create a past voice")
	}
	w.VoicePitches[current].PortaTargetHz = w.VoicePitches[current].BaseFreqHz * 2
	initITTonePortamentoMemory(w, &w.TrackMemories[0], 8)
	pastPitch, initial := w.VoicePitches[past].BaseFreqHz, w.VoicePitches[current].BaseFreqHz
	w.PendingCmds[0] = ModCommand{Note: 253, EffectCmd: 0x03}
	w.Clock.Tick = 0
	VoiceAllocationSystem(w)
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoicePitches[current].BaseFreqHz <= initial ||
		w.VoiceLives[past].ITFade || w.VoicePitches[past].BaseFreqHz != pastPitch {
		t.Fatal("fade or portamento affected the wrong voice")
	}
}
