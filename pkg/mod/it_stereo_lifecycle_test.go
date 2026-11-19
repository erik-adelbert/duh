package mod

import (
	"math"
	"testing"
)

func TestStereoVoiceReuseAndMix(t *testing.T) {
	assets := SongAssets{
		NumTracks: 2,
		Samples: []SampleAsset{
			{Data: []float32{0.25, 0.25, 0.25}, RightData: []float32{0.5, 0.5, 0.5}, C5Speed: 8363, Volume: 64},
			{Data: []float32{0.125, 0.125, 0.125, 0.125}, C5Speed: 8363, Volume: 64, LoopEnable: true, LoopEnd: 4},
		},
	}
	w := NewTracker(assets, 48000, 4)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	w.PendingCmds[1] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	stereo, mono := w.TrackCursors[0].ActiveVoice, w.TrackCursors[1].ActiveVoice
	for _, v := range []VoiceID{stereo, mono} {
		w.VoicePitches[v].StepIncrement = 1
		w.VoiceAmps[v].CurrentLeft, w.VoiceAmps[v].TargetLeft = 1, 1
		w.VoiceAmps[v].CurrentRight, w.VoiceAmps[v].TargetRight = 1, 1
	}
	ResamplerSystem(w, 4)
	out := make([]float32, 8)
	MixerAccumulationSystem(w, out, 4)
	for i := range 3 {
		if out[2*i] != 0.375 || out[2*i+1] != 0.625 {
			t.Fatalf("frame %d: got %g, %g", i, out[2*i], out[2*i+1])
		}
	}
	if out[6] != 0.125 || out[7] != 0.125 || w.VoiceLives[stereo].Active {
		t.Fatal("natural end did not leave only the mono voice")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	w.PendingCmds[1] = ModCommand{}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != stereo {
		t.Fatal("stopped stereo voice was not reused")
	}
	w.VoicePitches[stereo].StepIncrement = 1
	w.VoiceAmps[stereo].CurrentLeft, w.VoiceAmps[stereo].TargetLeft = 1, 1
	w.VoiceAmps[stereo].CurrentRight, w.VoiceAmps[stereo].TargetRight = 1, 1
	ResamplerSystem(w, 4)
	MixerAccumulationSystem(w, out, 4)
	for i, value := range out {
		if value != 0.25 {
			t.Fatalf("reused mono output sample %d = %g, want 0.25", i, value)
		}
	}
}

func TestStereoFilterChunkingAndAllocations(t *testing.T) {
	setup := func() *Tracker {
		sample := SampleAsset{
			Data: []float32{0, 0.02, 0.04, 0.06}, RightData: []float32{0.08, 0.04, 0.02, 0},
			C5Speed: 8363, Volume: 64, LoopEnable: true, LoopEnd: 4,
		}
		w := NewTracker(SongAssets{NumTracks: 1, Samples: []SampleAsset{sample}}, 48000, 64)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		w.VoicePitches[v].StepIncrement = 0.75
		w.VoiceAmps[v].CurrentLeft, w.VoiceAmps[v].TargetLeft = 1, 1
		w.VoiceAmps[v].CurrentRight, w.VoiceAmps[v].TargetRight = 1, 1
		w.VoiceFilters[v] = newITFilter(80, 90, 48000, true, false, false)
		w.VoiceRightFilters[v] = w.VoiceFilters[v]
		return w
	}
	whole, split := setup(), setup()
	a, b := make([]float32, 128), make([]float32, 128)
	ResamplerSystem(whole, 64)
	MixerAccumulationSystem(whole, a, 64)
	for i := 0; i < 64; i += 16 {
		ResamplerSystem(split, 16)
		MixerAccumulationSystem(split, b[i*2:(i+16)*2], 16)
	}
	for i := range a {
		if math.Abs(float64(a[i]-b[i])) > 1e-7 {
			t.Fatalf("sample %d: whole %g split %g", i, a[i], b[i])
		}
	}
	if got := testing.AllocsPerRun(100, func() {
		ResamplerSystem(split, 64)
		MixerAccumulationSystem(split, b, 64)
	}); got != 0 {
		t.Fatalf("stereo rendering allocated %g times per block", got)
	}
}
