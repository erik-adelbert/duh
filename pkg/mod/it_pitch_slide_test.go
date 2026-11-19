package mod

import (
	"math"
	"testing"
)

func TestOpenMPTITPitchSlides(t *testing.T) {
	tests := []struct {
		name   string
		effect byte
		arg    byte
		linear bool
		legacy bool
	}{
		{"fine down", 5, 0xf1, false, false},
		{"zero fine down", 5, 0xf0, false, false},
		{"zero extra fine down", 5, 0xe0, false, false},
		{"fine up", 6, 0xf8, false, false},
		{"extra fine down", 5, 0xe7, false, false},
		{"regular down", 5, 8, false, false},
		{"regular up", 6, 8, false, false},
		{"linear fine down", 5, 0xf1, true, false},
		{"linear extra fine up", 6, 0xe7, true, false},
		{"linear regular down", 5, 8, true, false},
		{"legacy fine down", 5, 0xf1, false, true},
		{"legacy regular up", 6, 8, false, true},
		{"legacy linear fine", 5, 0xf1, true, true},
		{"legacy linear regular", 6, 8, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testITPitchFixture([]byte{0x81, 11, 60, 1, tt.effect, tt.arg, 0}, 6, tt.linear, tt.legacy)
			baseline := append([]byte(nil), b...)
			baseline[344+4], baseline[344+5] = 0, 0
			refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			for tick := range 6 {
				want := referenceTickFrequency(t, ref, tick) / referenceTickFrequency(t, refBase, tick)
				got := referenceTickFrequency(t, actual, tick) / referenceTickFrequency(t, actualBase, tick)
				if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .001 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.6f, tracker %.6f", tick, want, got)
				}
			}
		})
	}
}

func TestOpenMPTITPitchSlideMemory(t *testing.T) {
	for _, tt := range []struct {
		name   string
		rows   []byte
		legacy bool
	}{
		{"E00 remembers EF4", []byte{0x81, 11, 60, 1, 5, 0xf4, 0, 0x81, 8, 5, 0, 0}, false},
		{"F00 remembers EF4", []byte{0x81, 11, 60, 1, 5, 0xf4, 0, 0x81, 8, 6, 0, 0}, false},
		{"memory before note", []byte{0x81, 8, 5, 0xf4, 0, 0x81, 11, 60, 1, 5, 0, 0}, false},
		{"legacy F00 remembers EF4", []byte{0x81, 11, 60, 1, 5, 0xf4, 0, 0x81, 8, 6, 0, 0}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testITPitchFixture(tt.rows, 2, true, tt.legacy)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			refBase, actualBase := referenceTickFrequency(t, ref, 3), referenceTickFrequency(t, actual, 3)
			for _, tick := range []int{2, 4, 5} {
				want, got := referenceTickFrequency(t, ref, tick)/refBase, referenceTickFrequency(t, actual, tick)/actualBase
				if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .001 {
					t.Errorf("tick %d pitch ratio: OpenMPT %.6f, tracker %.6f", tick, want, got)
				}
			}
		})
	}
}

func TestITPitchSlideTimingAndMemory(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LinearSlides: true},
		Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x02, EffectArg: 0xf1}
	PitchEffectSystem(w)
	if w.TrackMemories[0].LastPortaUp != 0xf1 || w.TrackMemories[1].LastPortaUp != 0 {
		t.Fatal("pitch slide memory not retained per track")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x02}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	before := w.VoicePitches[v].BaseFreqHz
	PitchEffectSystem(w)
	after := w.VoicePitches[v].BaseFreqHz
	if after >= before {
		t.Fatal("E00 did not recall fine slide on tick zero")
	}
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	if w.VoicePitches[v].BaseFreqHz != after {
		t.Fatal("fine slide repeated after tick zero")
	}
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x01}
	PitchEffectSystem(w)
	if w.VoicePitches[v].BaseFreqHz <= after {
		t.Fatal("F00 did not recall the opposite-direction slide")
	}
	before = w.VoicePitches[v].BaseFreqHz
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x02, EffectArg: 4}
	PitchEffectSystem(w)
	if w.VoicePitches[v].BaseFreqHz != before {
		t.Fatal("regular slide ran on tick zero")
	}
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	if w.VoicePitches[v].BaseFreqHz >= before {
		t.Fatal("regular slide did not run on tick one")
	}
}

func TestITPitchSlideEffectReuse(t *testing.T) {
	pattern, err := unpackITPattern([]byte{0x81, 8, 5, 0xf1, 0, 0x81, 0x80, 0, 0x81, 8, 5, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, arg := range []uint8{0xf1, 0xf1, 0} {
		cmd := pattern.Rows[row][0]
		if cmd.EffectCmd != 0x02 || cmd.EffectArg != arg {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
}
