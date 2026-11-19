package mod

import (
	"fmt"
	"testing"
)

func TestVoiceAllocationSpecialNoteWithoutVoice(t *testing.T) {
	for _, it := range []bool{false, true} {
		for _, note := range []uint8{253, 254, 255} {
			t.Run(fmt.Sprintf("IT=%t/note=%d", it, note), func(t *testing.T) {
				w := NewTracker(SongAssets{
					IT: it, NumTracks: 1,
					Samples: []SampleAsset{{Data: []float32{0, 0}, Volume: 64, C5Speed: 8363}},
				}, 48000, 1)
				w.PendingCmds[0] = ModCommand{Note: note, Instrument: 1}
				VoiceAllocationSystem(w)

				wantInstrument := uint8(1)
				if it && note == 253 {
					wantInstrument = 0
				}
				if got := w.TrackMemories[0].LastInstrument; got != wantInstrument {
					t.Errorf("instrument memory = %d, want %d", got, wantInstrument)
				}
				if got := w.TrackMemories[0].itSpecialNote; got != it {
					t.Errorf("special note memory = %t, want %t", got, it)
				}
				if w.TrackCursors[0].ActiveVoice != -1 {
					t.Fatal("special note unexpectedly started a voice")
				}
			})
		}
	}
}
