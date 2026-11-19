package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeVolumeSlide(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, old := range []bool{false, true} {
			for _, volume := range []byte{85, 86, 89, 95, 96, 99} {
				t.Run(fmt.Sprintf("legacy=%t/old=%t/volume=%d", legacy, old, volume), func(t *testing.T) {
					fixture := func(target byte, fade uint16) []byte {
						b := testITInstrumentWithRows([]byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 97, 0, 0x81, 5, 246, target, 0})
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
					for tick := 6; tick < 12; tick++ {
						ms := tick*20 + 15
						rb, ab := rmsWindow(refBase, ms, ms+3), rmsWindow(actualBase, ms, ms+3)
						rn, an := rmsWindow(refNoFade, ms, ms+3), rmsWindow(actualNoFade, ms, ms+3)
						if rb < .001 || ab < .001 || rn < .001 || an < .001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, ms, ms+3)/rb, rmsWindow(actual, ms, ms+3)/ab
						if math.IsNaN(got) || math.Abs(got-want) > .05 {
							t.Errorf("tick %d combined: OpenMPT %.3f, tracker %.3f", tick, want, got)
						}
						wantFade, gotFade := rmsWindow(ref, ms, ms+3)/rn, rmsWindow(actual, ms, ms+3)/an
						if math.IsNaN(gotFade) || math.Abs(gotFade-wantFade) > .05 {
							t.Errorf("tick %d fade: OpenMPT %.3f, tracker %.3f", tick, wantFade, gotFade)
						}
						if tick == 8 && volume != 85 && math.Abs(rn/rb-1) < .02 {
							t.Error("reference slide had no measurable effect")
						}
						if tick == 11 && wantFade > .95 {
							t.Error("reference fade had no measurable effect")
						}
					}
				})
			}
		}
	}
}

func TestITFadeVolumeSlideValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"up memory", []byte{0x81, 5, 246, 85, 0}, true},
		{"down four", []byte{0x81, 5, 246, 99, 0}, true},
		{"down maximum", []byte{0x81, 5, 246, 104, 0}, true},
		{"instrument", []byte{0x81, 7, 246, 1, 99, 0}, false},
		{"effect", []byte{0x81, 13, 246, 99, 5, 8, 0}, false},
		{"fine slide", []byte{0x81, 5, 246, 77, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 5, 253, 99, 0})); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}

func TestITFadeVolumeSlideVoiceIsolation(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 5, 246, 99, 0})
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
	pastVolume, currentVolume := w.VoiceAmps[past].Volume, w.VoiceAmps[current].Volume
	w.PendingCmds[0] = ModCommand{Note: 253, HasVolume: true, Volume: 99}
	w.Clock.Tick = 0
	VoiceAllocationSystem(w)
	VolumeEffectSystem(w)
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceAmps[current].Volume != currentVolume-4.0/64 ||
		w.VoiceLives[past].ITFade || w.VoiceAmps[past].Volume != pastVolume {
		t.Fatal("fade or volume slide affected the wrong voice")
	}
}
