package mod

import "testing"

func TestMPTMSharedEffectsOPLState(t *testing.T) {
	tests := []struct {
		name  string
		rows  []byte
		check func(*Tracker, VoiceID) bool
	}{
		{"K", []byte{0x81, 11, 60, 2, 8, 0x44, 0, 0x81, 8, 11, 0x02, 0},
			func(w *Tracker, v VoiceID) bool {
				return w.VoiceAmps[v].Volume < 1 && w.VoicePitches[v].VibratoOffset != 0
			}},
		{"L", []byte{0x81, 11, 60, 2, 7, 0x10, 0, 0x81, 9, 64, 12, 0x02, 0},
			func(w *Tracker, v VoiceID) bool {
				return w.VoiceAmps[v].Volume < 1 && w.VoicePitches[v].BaseFreqHz > noteToHz(60)
			}},
		{"N", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 14, 0x03, 0},
			func(w *Tracker, _ VoiceID) bool { return w.ITChannelVolume[0] == 61 }},
		{"P", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 16, 0x04, 0},
			func(w *Tracker, v VoiceID) bool { return w.VoiceAmps[v].Panning > 0 }},
		{"U", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 21, 0x8f, 0},
			func(w *Tracker, v VoiceID) bool { return w.VoicePitches[v].VibratoOffset != 0 }},
		{"W", []byte{0x81, 3, 60, 2, 0, 0x81, 8, 23, 0x03, 0},
			func(w *Tracker, _ VoiceID) bool { return w.ITGlobalVolume == 61 }},
		{"I", []byte{0x81, 11, 60, 2, 9, 0x13, 0, 0x81, 8, 9, 0, 0},
			func(w *Tracker, v VoiceID) bool { return w.VoiceAmps[v].itTremorMuted }},
		{"R", []byte{0x81, 15, 60, 2, 32, 18, 0x4f, 0, 0x81, 8, 18, 0, 0},
			func(w *Tracker, v VoiceID) bool { return w.VoiceAmps[v].itTremoloOffset != 0 }},
	}
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loaded, err := loadMPTM(mptSharedEffectFixture(t, tt.rows, flags))
			if err != nil {
				t.Fatal(err)
			}
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			buf := make([]float32, 256*2)
			audible := false
			for remaining := 3*960 + 1; remaining > 0; {
				n := min(remaining, 256)
				if got := w.Read(buf[:n*2], n); got != n {
					t.Fatalf("short render: %d", got)
				}
				for _, sample := range buf[:n*2] {
					audible = audible || sample != 0
				}
				remaining -= n
			}
			v := w.TrackCursors[0].ActiveVoice
			if v < 0 || !w.VoiceLives[v].Active || w.VoiceSamples[v].oplChannel == 0 || !audible {
				t.Fatal("expected an active, audible OPL voice")
			}
			if !tt.check(w, v) {
				t.Fatalf("effect did not update OPL voice state: amp=%+v pitch=%+v channel volume=%d global volume=%d",
					w.VoiceAmps[v], w.VoicePitches[v], w.ITChannelVolume[0], w.ITGlobalVolume)
			}
		})
	}
}
