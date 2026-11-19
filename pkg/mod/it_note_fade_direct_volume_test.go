package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeDirectVolume(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, volume := range []byte{0, 5, 24, 64} {
			for _, fadeout := range []uint16{0, 32} {
				t.Run(fmt.Sprintf("legacy=%t/volume=%d/fadeout=%d", legacy, volume, fadeout), func(t *testing.T) {
					fixture := func(target byte, fade uint16) []byte {
						b := testITInstrumentWithRows([]byte{0x81, 3, 60, 1, 0, 0x81, 5, 246, target, 0})
						b[50], b[44] = 6, b[44]|8
						if legacy {
							binary.LittleEndian.PutUint16(b[40:], 0x217)
							binary.LittleEndian.PutUint16(b[42:], 0x200)
						}
						h := b[len(b)-554:]
						h[17], h[304] = 0, 0
						binary.LittleEndian.PutUint16(h[20:], fade)
						copy(b[512:512+8192], testITPitchFixture(nil, 6, true, legacy)[512:512+8192])
						binary.LittleEndian.PutUint32(b[256+60:], 24000)
						binary.LittleEndian.PutUint32(b[256+56:], 3840)
						return b
					}
					ref, actual := referencePCMWithChunks(t, fixture(volume, fadeout), "2", 360, 97)
					refBase, actualBase := referencePCMWithChunks(t, fixture(64, 0), "2", 360, 97)
					_, whole := referencePCMWithChunks(t, fixture(volume, fadeout), "2", 360, 0)
					for i := range actual {
						if actual[i] != whole[i] {
							t.Fatalf("chunk-dependent PCM at sample %d", i)
						}
					}
					for tick := 6; tick < 16; tick++ {
						ms := tick*20 + 15
						rb, ab := rmsWindow(refBase, ms, ms+3), rmsWindow(actualBase, ms, ms+3)
						if rb < .001 || ab < .001 {
							t.Fatal("silent comparison baseline")
						}
						want, got := rmsWindow(ref, ms, ms+3)/rb, rmsWindow(actual, ms, ms+3)/ab
						if math.IsNaN(got) || math.Abs(got-want) > .05 {
							t.Errorf("tick %d: OpenMPT %.3f, tracker %.3f", tick, want, got)
						}
						if tick == 6 && math.Abs(want-float64(volume)/64) > .05 {
							t.Errorf("reference did not apply direct volume: %.3f", want)
						}
						if tick == 15 && volume > 0 && fadeout > 0 && want >= float64(volume)/64*.95 {
							t.Errorf("reference did not fade: %.3f", want)
						}
					}
				})
			}
		}
	}
}

func TestITFadeDirectVolumeValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"zero", []byte{0x81, 5, 246, 0, 0}, true},
		{"five", []byte{0x81, 5, 246, 5, 0}, true},
		{"maximum", []byte{0x81, 5, 246, 64, 0}, true},
		{"instrument", []byte{0x81, 7, 246, 1, 5, 0}, false},
		{"panning", []byte{0x81, 5, 246, 160, 0}, false},
		{"other effect", []byte{0x81, 13, 246, 5, 6, 8, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 5, 253, 5, 0})); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}

func TestITFadeDirectVolumeVoiceIsolation(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 5, 246, 5, 0})
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
	pastVolume := w.VoiceAmps[past].Volume
	w.PendingCmds[0] = ModCommand{Note: 253, HasVolume: true, Volume: 5}
	VoiceAllocationSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceAmps[current].Volume != 5.0/64 ||
		w.VoiceLives[past].ITFade || w.VoiceAmps[past].Volume != pastVolume {
		t.Fatal("fade or direct volume affected the wrong voice")
	}
}
