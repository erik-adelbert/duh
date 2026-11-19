package mod

import (
	"fmt"
	"testing"
)

func TestMPTFineInstrumentPanningPaths(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(fmt.Sprintf("restore=%t", restore), func(t *testing.T) {
			loaded, err := loadMPTM(mptInstrumentPanFixture(t, 103, true))
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITCompatibility.itRestoreChannelPan = restore
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			if w.Read(make([]float32, 512), 256) != 256 {
				t.Fatal("short render")
			}
			voice := w.TrackCursors[0].ActiveVoice
			if voice < 0 {
				t.Fatal("no active voice")
			}
			want := float32(103)/128 - 1
			if got := w.VoiceAmps[voice].Panning; got != want {
				t.Fatalf("initial pan %g, want %g", got, want)
			}
			cursor := &w.TrackCursors[0]
			w.applyMPTNoteDefaultPan(cursor, voice, loaded.Assets.ITInstruments[1].normalizedPanning())
			if cursor.Panning != want || w.VoiceAmps[voice].Panning != want {
				t.Fatalf("reset pan %g/%g, want %g", cursor.Panning, w.VoiceAmps[voice].Panning, want)
			}
		})
	}
}
