package mod

import "testing"

func TestEndedVoiceReuseDoesNotAffectOtherTrack(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2,
		Samples: []SampleAsset{{Data: []float32{1, 0.5}, C5Speed: 8363, Volume: 64}},
	}, 48000, 4)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.VoicePitches[old].StepIncrement = 1
	ResamplerSystem(w, 4)
	MixerAccumulationSystem(w, make([]float32, 8), 4)
	if w.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("natural end left a stale track voice")
	}

	w.PendingCmds[0] = ModCommand{}
	w.PendingCmds[1] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	if got := w.TrackCursors[1].ActiveVoice; got != old {
		t.Fatalf("second track voice = %d, want reused voice %d", got, old)
	}
	w.PendingCmds[0] = ModCommand{Note: 254}
	w.PendingCmds[1] = ModCommand{}
	VoiceAllocationSystem(w)
	if !w.VoiceLives[old].Active || w.TrackCursors[1].ActiveVoice != old {
		t.Fatal("cut on the first track stopped the second track's voice")
	}
}

func TestStoppingPastNotePreservesForegroundVoice(t *testing.T) {
	assets := SongAssets{
		IT: true, NumTracks: 1,
		Samples:       []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
		ITInstruments: []ITInstrument{{NewNoteAction: 1}},
	}
	for i := range assets.ITInstruments[0].Samples {
		assets.ITInstruments[0].Samples[i] = 1
		assets.ITInstruments[0].Notes[i] = uint8(i)
	}
	w := NewTracker(assets, 48000, 4)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	past := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if past == current || !w.VoiceLives[past].ITPastNote {
		t.Fatal("NNA continue did not create a past note")
	}
	w.stopVoice(past)
	if w.VoiceLives[past].Active || w.TrackCursors[0].ActiveVoice != current || !w.VoiceLives[current].Active {
		t.Fatal("stopping a past note affected the foreground voice")
	}
}
