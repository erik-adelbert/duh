package mod

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMVolume203(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"vibrato memory", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 4, 203, 0}},
		{"no memory", []byte{0x81, 7, 60, 1, 203, 0, 0x81, 4, 203, 0}},
		{"new note", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 7, 60, 1, 203, 0}},
		{"volume slide memory", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 4, 88, 0, 0x81, 4, 203, 0}},
		{"pitch slide memory", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 4, 108, 0, 0x81, 4, 203, 0}},
		{"fine vibrato memory", []byte{0x81, 11, 60, 1, 21, 0x8f, 0, 0x81, 4, 203, 0}},
		{"waveform", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 12, 203, 19, 0x31, 0}},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, shared := range []bool{false, true} {
			for _, precise := range []bool{false, true} {
				for _, old := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/shared=%t/precise=%t/old=%t", tt.name, shared, precise, old), func(t *testing.T) {
						flags := all
						if shared {
							flags &^= 1 << mptITVolColMemory
						}
						if !precise {
							flags &^= 1 << mptITVibratoTremoloPanbrello
						}
						b := mptSharedEffectFixture(t, tt.rows, flags)
						b[50] = 6
						if old {
							b[44] |= 0x10
						}
						ref, actual := mptSharedEffectRenderForDuration(t, b, 360)
						for tick := range 18 {
							want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
							if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got/want-1) > .004 {
								t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
							}
						}
					})
				}
			}
		}
	}
}

func TestMPTMVolume203OPL(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	rows := []byte{0x81, 11, 60, 2, 8, 0x8f, 0, 0x81, 4, 203, 0}
	loaded, err := loadMPTM(mptSharedEffectFixture(t, rows, flags))
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 512)
	vibrato := false
	for frames := 0; frames < 4*960; {
		n := min(256, 4*960-frames)
		if got := w.Read(buf[:n*2], n); got != n {
			t.Fatalf("short render: %d", got)
		}
		frames += n
		voice := w.TrackCursors[0].ActiveVoice
		if frames > 2*960 && voice >= 0 && w.VoiceSamples[voice].oplChannel != 0 && w.VoicePitches[voice].VibratoOffset != 0 {
			vibrato = true
		}
	}
	voice := w.TrackCursors[0].ActiveVoice
	if voice < 0 || w.VoiceSamples[voice].oplChannel == 0 || !vibrato {
		t.Fatal("volume 203 did not apply vibrato to the OPL voice")
	}
}

func TestMPTMVolume203Validation(t *testing.T) {
	if _, err := unpackITPatternMode([]byte{0x81, 4, 203, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackITPatternMode([]byte{0x81, 12, 203, 8, 0x82, 0}, 1, true); err == nil || !strings.Contains(err.Error(), "volume vibrato") {
		t.Fatalf("expected combination rejection, got %v", err)
	}
}
