package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITFineVolumeSlides(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"up and reuse", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 68, 0, 0x81, 4, 65, 0}},
		{"down and reuse", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 79, 0, 0x81, 4, 75, 0}},
		{"opposite direction", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 69, 0, 0x81, 4, 75, 0}},
		{"regular to fine", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 88, 0, 0x81, 4, 75, 0}},
		{"fine to regular", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 68, 0, 0x81, 4, 95, 0}},
		{"combined effect slide", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 12, 68, 4, 2, 0}},
		{"effect then fine zero", []byte{0x81, 15, 60, 1, 32, 4, 0xf3, 0, 0x81, 4, 65, 0}},
		{"fine then effect zero", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 68, 0, 0x81, 8, 4, 0, 0}},
		{"fine and fine effect", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 12, 68, 4, 0xf2, 0, 0x81, 4, 65, 0}},
		{"fine zero without memory", []byte{0x81, 7, 60, 1, 65, 0}},
		{"upper clamp", []byte{0x81, 7, 60, 1, 64, 0, 0x81, 4, 74, 0}},
		{"lower clamp", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 84, 0, 0x81, 4, 84, 0}},
		{"memory before note", []byte{0x81, 4, 69, 0, 0x81, 7, 60, 1, 75, 0}},
		{"delayed note", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 15, 60, 1, 79, 19, 0xd1, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				b[256+19] = 32
				baseline := testITPitchFixture([]byte{0x81, 7, 60, 1, 32, 0}, 2, true, legacy)
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
				for _, ms := range []int{8, 28, 48, 68, 88, 108} {
					if tt.name == "memory before note" && ms < 40 {
						continue
					}
					if tt.name == "memory before note" && ms == 48 || tt.name == "delayed note" && ms == 68 {
						continue
					}
					want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
					got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
						t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}

func TestITFineVolumeSlideTimingAndMemory(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			w := NewTracker(SongAssets{
				IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy},
				Samples: []SampleAsset{{Data: make([]float32, 16), Volume: 32, C5Speed: 48000}},
			}, 48000, 16)
			w.Clock.Speed = 3
			w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 69}
			VolumeEffectSystem(w)
			if w.TrackMemories[0].ITVolSlide != 4 || w.TrackMemories[1].ITVolSlide != 0 {
				t.Fatal("fine volume memory not retained or not isolated")
			}
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, HasVolume: true, Volume: 75}
			VoiceAllocationSystem(w)
			voice := w.TrackCursors[0].ActiveVoice
			VolumeEffectSystem(w)
			if got := w.VoiceAmps[voice].Volume * 64; got != 28 {
				t.Fatalf("recalled fine slide volume = %g, want 28", got)
			}
			w.Clock.Tick = 1
			VolumeEffectSystem(w)
			if got := w.VoiceAmps[voice].Volume * 64; got != 28 {
				t.Fatalf("fine slide repeated on a later tick: %g", got)
			}
			w.Clock.Tick = 0
			w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 74, EffectCmd: 0x15, EffectArg: 0xd3}
			VolumeEffectSystem(w)
			if got := w.VoiceAmps[voice].Volume * 64; got != 28 || w.TrackMemories[0].ITVolSlide != 4 {
				t.Fatalf("out-of-range delayed slide ran or changed memory: volume=%g memory=%d", got, w.TrackMemories[0].ITVolSlide)
			}
			w.Clock.Speed = 1
			w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 74}
			VolumeEffectSystem(w)
			if got := w.VoiceAmps[voice].Volume * 64; got != 37 {
				t.Fatalf("speed-one fine slide volume = %g, want 37", got)
			}
		})
	}
}

func TestITFineVolumeSlidePackedReuse(t *testing.T) {
	pattern, err := unpackITPattern([]byte{0x81, 4, 66, 0, 0x81, 0x40, 0, 0x81, 4, 76, 0, 0x81, 4, 84, 0}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint8{66, 66, 76, 84} {
		if got := pattern.Rows[i][0]; !got.HasVolume || got.Volume != want {
			t.Fatalf("row %d: %+v, want %d", i, got, want)
		}
	}

}

func TestITFineVolumeSlideEffectMemoryWithoutVoice(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: true},
	}, 48000, 1)
	w.Clock.Speed = 2
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x0a, EffectArg: 0x3f}
	VolumeEffectSystem(w)
	if got := w.TrackMemories[0].ITFineVolSlide; got != 3 {
		t.Fatalf("fine effect without voice memory = %d, want 3", got)
	}
	w.TrackMemories[0].ITFineVolSlide = 7
	w.PendingCmds[0].EffectArg = 0
	VolumeEffectSystem(w)
	if got := w.TrackMemories[0].ITFineVolSlide; got != 3 {
		t.Fatalf("D00 recalled fine memory = %d, want 3", got)
	}
	if got := w.TrackMemories[1].ITFineVolSlide; got != 0 {
		t.Fatalf("fine memory leaked to another track: %d", got)
	}
	w.PendingCmds[0].EffectArg = 0xf4
	VolumeEffectSystem(w)
	if got := w.TrackMemories[0].ITFineVolSlide; got != 4 {
		t.Fatalf("fine down effect memory = %d, want 4", got)
	}
}
