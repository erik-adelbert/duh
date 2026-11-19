package mod

import "testing"

func TestMPTChannelPanRestoreState(t *testing.T) {
	inst := ITInstrument{GlobalVolume: 128, NewNoteAction: 1, HasPanning: true, Panning: 0}
	for i := range inst.Samples {
		inst.Samples[i], inst.Notes[i] = 1, uint8(i)
	}
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITInstruments: []ITInstrument{inst},
		ITCompatibility: ITCompatibility{itRestoreChannelPan: true},
		Samples:         []SampleAsset{{Data: []float32{1, 1}, C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	cursor := &w.TrackCursors[0]
	cursor.Panning, cursor.Surround = .5, true
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := cursor.ActiveVoice
	if w.VoiceAmps[old].Panning != -1 || w.VoiceAmps[old].Surround || !cursor.itHasRestorePan {
		t.Fatal("instrument panning was not applied temporarily")
	}
	w.Assets.ITInstruments[0].HasPanning = false
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	current := cursor.ActiveVoice
	if current == old || w.VoiceAmps[old].Panning != -1 || !w.VoiceLives[old].Active ||
		w.VoiceAmps[current].Panning != .5 || !w.VoiceAmps[current].Surround {
		t.Fatal("next note did not restore channel pan and surround independently of past voice")
	}
	w.Assets.ITInstruments[0].HasPanning = true
	w.PendingCmds[0] = ModCommand{Note: 63}
	VoiceAllocationSystem(w)
	current = cursor.ActiveVoice
	setTrackPanning(w, 0, current, .25)
	w.Assets.ITInstruments[0].HasPanning = false
	w.PendingCmds[0] = ModCommand{Note: 64}
	VoiceAllocationSystem(w)
	current = cursor.ActiveVoice
	if w.VoiceAmps[current].Panning != .25 || w.VoiceAmps[current].Surround || cursor.itHasRestorePan {
		t.Fatal("explicit panning command did not replace pending restoration")
	}
}
