package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeFineVolumeTempo(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, old := range []bool{false, true} {
			for _, volume := range []byte{65, 66, 75, 76, 84} {
				for _, tempo := range []byte{0x90, 0x7d} {
					t.Run(fmt.Sprintf("legacy=%t/old=%t/vol=%d/T%02X", legacy, old, volume, tempo), func(t *testing.T) {
						fixture := func(target byte, fade uint16) []byte {
							rows := []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 77, 0,
								0x81, 13, 246, target, 20, tempo, 0, 0}
							if target == 32 {
								rows = []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 77, 0,
									0x81, 8, 20, tempo, 0, 0}
							}
							b := testITInstrumentWithRows(rows)
							b[50], b[44] = 3, b[44]|8
							if old {
								b[44] |= 0x10
							}
							if legacy {
								binary.LittleEndian.PutUint16(b[40:], 0x217)
								binary.LittleEndian.PutUint16(b[42:], 0x200)
							}
							h := b[len(b)-554:]
							h[17], h[304] = 0, 0
							binary.LittleEndian.PutUint16(h[20:], fade)
							copy(b[512:512+8192], testITPitchFixture(nil, 3, true, legacy)[512:512+8192])
							binary.LittleEndian.PutUint32(b[256+60:], 24000)
							binary.LittleEndian.PutUint32(b[256+56:], 3840)
							return b
						}
						ref, actual := referencePCMWithChunks(t, fixture(volume, 32), "2", 240, 97)
						refBase, actualBase := referencePCMWithChunks(t, fixture(32, 0), "2", 240, 97)
						refNoFade, actualNoFade := referencePCMWithChunks(t, fixture(volume, 0), "2", 240, 97)
						_, whole := referencePCMWithChunks(t, fixture(volume, 32), "2", 240, 0)
						for i := range actual {
							if actual[i] != whole[i] {
								t.Fatalf("chunk-dependent PCM at sample %d", i)
							}
						}
						for _, ms := range []int{135, 155, 175, 195, 215} {
							rb, ab := rmsWindow(refBase, ms, ms+5), rmsWindow(actualBase, ms, ms+5)
							rn, an := rmsWindow(refNoFade, ms, ms+5), rmsWindow(actualNoFade, ms, ms+5)
							if rb < .001 || ab < .001 || rn < .001 || an < .001 {
								t.Fatal("silent comparison baseline")
							}
							want, got := rmsWindow(ref, ms, ms+5)/rb, rmsWindow(actual, ms, ms+5)/ab
							if math.IsNaN(got) || math.Abs(got-want) > .05 {
								t.Errorf("%dms combined: OpenMPT %.3f, tracker %.3f", ms, want, got)
							}
							wantFade, gotFade := rmsWindow(ref, ms, ms+5)/rn, rmsWindow(actual, ms, ms+5)/an
							if math.IsNaN(gotFade) || math.Abs(gotFade-wantFade) > .05 {
								t.Errorf("%dms fade: OpenMPT %.3f, tracker %.3f", ms, wantFade, gotFade)
							}
							if ms == 215 && wantFade > .95 {
								t.Error("reference note fade did not measurably reduce volume")
							}
							if ms == 155 && math.Abs(rn/rb-1) < .02 {
								t.Error("reference fine slide had no measurable effect")
							}
						}
					})
				}
			}
		}
	}
}

func TestITFadeFineVolumeTempoValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"fine down T90", []byte{0x81, 13, 246, 76, 20, 0x90, 0}, true},
		{"fine up T20", []byte{0x81, 13, 246, 66, 20, 0x20, 0}, true},
		{"tempo slide", []byte{0x81, 13, 246, 76, 20, 0x11, 0}, false},
		{"instrument", []byte{0x81, 15, 246, 1, 76, 20, 0x90, 0}, false},
		{"normal slide", []byte{0x81, 13, 246, 99, 20, 0x90, 0}, false},
		{"other effect", []byte{0x81, 13, 246, 76, 5, 8, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 13, 253, 76, 20, 0x90, 0})); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}

func TestITFadeFineVolumeTempoState(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 13, 246, 76, 20, 0x90, 0})
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 32)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	past, current := VoiceID(0), w.TrackCursors[0].ActiveVoice
	if past == current || !w.VoiceLives[past].ITPastNote {
		t.Fatal("fixture did not create a past voice")
	}
	before, pastVolume := w.VoiceAmps[current].Volume, w.VoiceAmps[past].Volume
	w.PendingCmds[0] = ModCommand{Note: 253, HasVolume: true, Volume: 76, EffectCmd: 0x20, EffectArg: 0x90}
	w.Clock.Tick, w.Clock.Speed, w.Clock.Tempo = 0, 3, 125
	VoiceAllocationSystem(w)
	VolumeEffectSystem(w)
	GlobalFlowSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceAmps[current].Volume != before-1.0/64 ||
		w.VoiceLives[past].ITFade || w.VoiceAmps[past].Volume != pastVolume || w.Clock.Tempo != 144 {
		t.Fatalf("fade, fine slide or tempo state incorrect: tempo=%g", w.Clock.Tempo)
	}
}
