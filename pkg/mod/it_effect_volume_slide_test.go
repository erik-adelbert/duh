package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITEffectVolumeSlide(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"down", []byte{0x81, 15, 60, 1, 48, 4, 8, 0}},
		{"up", []byte{0x81, 15, 60, 1, 24, 4, 0x20, 0}},
		{"fine down", []byte{0x81, 15, 60, 1, 48, 4, 0xf3, 0}},
		{"fine up", []byte{0x81, 15, 60, 1, 24, 4, 0x3f, 0}},
		{"mixed nibbles", []byte{0x81, 15, 60, 1, 32, 4, 0x23, 0}},
		{"down 15", []byte{0x81, 15, 60, 1, 48, 4, 0x0f, 0}},
		{"up 15", []byte{0x81, 15, 60, 1, 16, 4, 0xf0, 0}},
		{"fine both 15", []byte{0x81, 15, 60, 1, 24, 4, 0xff, 0}},
		{"memory", []byte{0x81, 15, 60, 1, 48, 4, 3, 0, 0x81, 8, 4, 0, 0}},
		{"memory before note", []byte{0x81, 8, 4, 3, 0, 0x81, 7, 60, 1, 48, 0, 0x81, 8, 4, 0, 0}},
		{"volume column then effect", []byte{0x81, 7, 60, 1, 48, 0, 0x81, 4, 98, 0, 0x81, 8, 4, 0, 0}},
		{"effect then volume column", []byte{0x81, 15, 60, 1, 48, 4, 3, 0, 0x81, 4, 95, 0}},
		{"both columns", []byte{0x81, 15, 60, 1, 98, 4, 0x20, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%v/old=%v", tt.name, legacy, old), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 2, true, legacy)
					if old {
						b[44] |= 0x10
					}
					baseline := testITPitchFixture([]byte{0x81, 7, 60, 1, 48, 0}, 2, true, legacy)
					if old {
						baseline[44] |= 0x10
					}
					ref, actual := referencePCMWithInterpolation(t, b, "2")
					refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
					for _, ms := range []int{8, 28, 48, 68, 88, 108} {
						want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
						got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
						if tt.name == "memory before note" && ms < 60 {
							continue
						}
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
							t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITEffectVolumeSlideMemoryAndIsolation(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2,
		Samples: []SampleAsset{{Data: make([]float32, 16), Volume: 32, C5Speed: 48000}},
	}, 48000, 16)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x0a, EffectArg: 3}
	VolumeEffectSystem(w)
	if w.TrackMemories[0].LastVolSlide != 3 || w.TrackMemories[1].LastVolSlide != 0 {
		t.Fatal("effect memory must be retained without a voice and isolated per track")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x0a}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.VoiceAmps[voice].Volume != 29.0/64 {
		t.Fatalf("recalled slide volume = %g", w.VoiceAmps[voice].Volume)
	}
}

func TestITEffectVolumeSlideAtSpeedOne(t *testing.T) {
	for _, tt := range []struct {
		arg  uint8
		want float32
	}{
		{0x08, 32}, {0x20, 32}, {0xf2, 30}, {0x2f, 34}, {0x0f, 17}, {0xf0, 47},
	} {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 1,
			Samples: []SampleAsset{{Data: make([]float32, 16), Volume: 32, C5Speed: 48000}},
		}, 48000, 16)
		w.Clock.Speed = 1
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x0a, EffectArg: tt.arg}
		VoiceAllocationSystem(w)
		VolumeEffectSystem(w)
		voice := w.TrackCursors[0].ActiveVoice
		if got := w.VoiceAmps[voice].Volume * 64; got != tt.want {
			t.Errorf("D%02X: volume %g, want %g", tt.arg, got, tt.want)
		}
	}
}
