package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITFadeVolumePitchSlide(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				for _, arg := range []byte{0, 0x08, 0xe2, 0xf2} {
					t.Run(fmt.Sprintf("legacy=%t/linear=%t/old=%t/E%02X", legacy, linear, old, arg), func(t *testing.T) {
						fixture := func(fadeout uint16, volume byte) []byte {
							b := testITInstrument()
							b[50] = 6
							if linear {
								b[44] |= 8
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
							copy(b[512:512+8192], testITPitchFixture(nil, 6, linear, legacy)[512:512+8192])
							binary.LittleEndian.PutUint32(b[256+60:], 24000)
							binary.LittleEndian.PutUint32(b[256+56:], 3840)
							rows := append([]byte{0x81, 11, 60, 1, 5, 0x10, 0, 0x81, 13, 246, volume, 5, arg, 0}, make([]byte, 62)...)
							binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
							copy(b[344:], rows)
							return b
						}
						ref, actual := referencePCMWithChunks(t, fixture(32, 24), "2", 360, 97)
						refBase, actualBase := referencePCMWithChunks(t, fixture(0, 64), "2", 360, 97)
						for tick := 6; tick < 16; tick++ {
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
								t.Errorf("tick %d volume and fade: OpenMPT %.3f, tracker %.3f", tick, want, got)
							}
							if tick == 15 && want > .35 {
								t.Errorf("volume/fade did not measurably reduce reference: %.3f", want)
							}
						}
					})
				}
			}
		}
	}
}

func TestITFadeVolumePitchSlideValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  []byte
		ok   bool
	}{
		{"E08 volume 24", []byte{0x81, 13, 246, 24, 5, 8, 0}, true},
		{"E00 volume 0", []byte{0x81, 13, 246, 0, 5, 0, 0}, true},
		{"E08 volume 64", []byte{0x81, 13, 246, 64, 5, 8, 0}, true},
		{"instrument", []byte{0x81, 15, 246, 1, 24, 5, 8, 0}, false},
		{"volume effect", []byte{0x81, 13, 246, 76, 5, 8, 0}, false},
		{"panning", []byte{0x81, 13, 246, 160, 5, 8, 0}, false},
		{"other effect", []byte{0x81, 13, 246, 24, 6, 8, 0}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadIT(testITInstrumentWithRows(tt.row))
			if tt.ok && err != nil || !tt.ok && (err == nil || !strings.Contains(err.Error(), "note fade")) {
				t.Fatalf("got %v", err)
			}
		})
	}
	b := mptS70Fixture(t, []byte{0x81, 13, 253, 24, 5, 8, 0})
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "note fade") {
		t.Errorf("MPTM combination: %v", err)
	}
}

func TestITFadeVolumePitchSlideVoiceIsolation(t *testing.T) {
	b := testITInstrumentWithRows([]byte{0x81, 13, 246, 24, 5, 8, 0})
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
	w.PendingCmds[0] = ModCommand{Note: 253, HasVolume: true, Volume: 24, EffectCmd: 0x02, EffectArg: 8}
	VoiceAllocationSystem(w)
	if !w.VoiceLives[current].ITFade || w.VoiceAmps[current].Volume != 24.0/64 ||
		w.VoiceLives[past].ITFade || w.VoiceAmps[past].Volume != pastVolume {
		t.Fatal("fade or direct volume affected the wrong voice")
	}
}
