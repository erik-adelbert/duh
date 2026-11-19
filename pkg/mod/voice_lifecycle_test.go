package mod

import "testing"

func TestVoiceCutReleaseAndReuse(t *testing.T) {
	assets := SongAssets{
		IT: true, NumTracks: 1, ITChannelVolume: [MaxTracks]uint8{64},
		Samples:       []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
		ITInstruments: []ITInstrument{{NewNoteAction: 1, Fadeout: 256}},
	}
	for i := range assets.ITInstruments[0].Samples {
		assets.ITInstruments[0].Samples[i] = 1
		assets.ITInstruments[0].Notes[i] = uint8(i)
	}
	w := NewTracker(assets, 48000, 32)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if old == current || !w.VoiceLives[old].ITPastNote || !w.VoiceLives[old].Active {
		t.Fatal("NNA continue did not retain the old voice")
	}
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	if !w.VoiceLives[current].Active || !w.VoiceLives[current].KeyOff || !w.VoiceLives[current].ITFade {
		t.Fatal("key-off did not release the current instrument voice")
	}
	for range 4 {
		VoiceLifecycleSystem(w)
	}
	if w.VoiceLives[current].Active || !w.VoiceLives[old].Active {
		t.Fatal("fade did not stop only the released voice")
	}
	w.PendingCmds[0] = ModCommand{Note: 63, Instrument: 1}
	VoiceAllocationSystem(w)
	reused := w.TrackCursors[0].ActiveVoice
	if reused != current || !w.VoiceLives[reused].Active || w.VoiceLives[reused].KeyOff {
		t.Fatal("stopped voice was not reinitialized on reuse")
	}
	w.PendingCmds[0] = ModCommand{Note: 254}
	VoiceAllocationSystem(w)
	if w.VoiceLives[reused].Active || w.TrackCursors[0].ActiveVoice != -1 || !w.VoiceLives[old].Active {
		t.Fatal("cut did not stop only the foreground voice")
	}
}

func TestVoiceStartReusesScratchBuffersWithoutAllocation(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1,
		Samples: []SampleAsset{{Data: make([]float32, 32), C5Speed: 8363, Volume: 64}},
	}, 48000, 32)
	left := &w.VoiceSamples[0].ScratchBuf[0]
	allocs := testing.AllocsPerRun(100, func() {
		if !w.startSampleVoice(0, ModCommand{Note: 61, Instrument: 1}, 0, 61, 0) {
			t.Fatal("voice allocation failed")
		}
	})
	if allocs != 0 {
		t.Fatalf("voice start allocated %g times", allocs)
	}
	voice := w.TrackCursors[0].ActiveVoice
	if voice != 0 || &w.VoiceSamples[voice].ScratchBuf[0] != left {
		t.Fatal("voice start did not reuse its scratch buffer")
	}
}

func TestPCMNaturalEndStopsAfterMix(t *testing.T) {
	w := NewTracker(SongAssets{
		NumTracks: 1,
		Samples:   []SampleAsset{{Data: []float32{1, 0.5}, C5Speed: 8363, Volume: 64}},
	}, 48000, 4)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.VoicePitches[voice].StepIncrement = 1
	w.VoiceAmps[voice].TargetLeft = 1
	ResamplerSystem(w, 4)
	if !w.VoiceLives[voice].Active || !w.VoiceSamples[voice].Ended {
		t.Fatal("PCM voice ended before its final samples were mixed")
	}

	out := make([]float32, 8)
	MixerAccumulationSystem(w, out, 4)
	if w.VoiceLives[voice].Active {
		t.Fatal("ended PCM voice remained active after mixing")
	}

	if out[2] == 0 {
		t.Fatal("last PCM sample was not mixed")
	}
}
