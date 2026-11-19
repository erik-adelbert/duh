package mod

import "testing"

func TestITPlaybackVolumeState(t *testing.T) {
	for _, tt := range []struct {
		name       string
		instrument int
		legacy     bool
	}{
		{"sample", 0, false},
		{"instrument", 1, false},
		{"legacy instrument", 1, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assets := SongAssets{
				IT: true, ITMixGain: 1, ITGlobalVolume: 128,
				ITChannelVolume: [MaxTracks]uint8{64, 64},
				ITCompatibility: ITCompatibility{LegacyModPlug: tt.legacy},
				Samples:         []SampleAsset{{ITGlobalVolume: 64}},
			}
			w := NewTracker(assets, 48000, 1)
			other := NewTracker(assets, 48000, 1)
			for voice, track := range []TrackID{0, 0, 1} {
				w.VoiceLives[voice] = VoiceLifecycle{
					Active: true, ParentTrack: track, FadeOutVol: 1,
					ITInstrument: tt.instrument, ITEnvelopeVolume: 1,
				}
				w.VoiceAmps[voice] = AmplitudeState{Volume: 1, ITInstrumentVolume: 1}
			}
			if got := voiceVolume(w, 0, 0); got != 1 {
				t.Fatalf("initial volume = %g, want 1", got)
			}
			w.ITChannelVolume[0] = 32
			for voice, want := range []float32{.5, .5, 1} {
				if got := voiceVolume(w, voice, 0); got != want {
					t.Errorf("voice %d after channel change: got %g, want %g", voice, got, want)
				}
			}
			w.ITGlobalVolume = 64
			for voice, want := range []float32{.25, .25, .5} {
				if got := voiceVolume(w, voice, 0); got != want {
					t.Errorf("voice %d after global change: got %g, want %g", voice, got, want)
				}
			}
			if w.Assets.ITGlobalVolume != 128 || w.Assets.ITChannelVolume[0] != 64 ||
				other.ITGlobalVolume != 128 || other.ITChannelVolume[0] != 64 {
				t.Fatal("playback state changed initial assets or another world")
			}
		})
	}
}
