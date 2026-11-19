package mod

import "testing"

func TestVoiceAllocationExhaustion(t *testing.T) {
	for _, it := range []bool{false, true} {
		w := NewTracker(SongAssets{
			IT: it, NumTracks: 1, ITMixGain: 1,
			Samples: []SampleAsset{{ITGlobalVolume: 64}},
		}, 48000, 256)
		w.ITChannelVolume[0], w.ITGlobalVolume = 64, 128
		for i := range MaxVoices {
			w.VoiceLives[i] = VoiceLifecycle{Active: true, ParentTrack: 0, FadeOutVol: 1}
			w.VoiceAmps[i].Volume = 1
		}
		w.TrackCursors[0].ActiveVoice = 0
		w.VoiceLives[5].ITEnvelopeTick = 10
		w.VoiceLives[9].ITEnvelopeTick = 20
		got := w.allocVoice()
		if !it && got != -1 {
			t.Fatalf("non-IT allocation reclaimed voice %d", got)
		}
		if it && got != 9 {
			t.Fatalf("IT equal-priority allocation reclaimed voice %d, want oldest envelope 9", got)
		}
		if !w.VoiceLives[0].Active || w.TrackCursors[0].ActiveVoice != 0 {
			t.Fatal("foreground voice was changed")
		}
	}
}
