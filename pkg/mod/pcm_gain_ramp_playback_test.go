package mod

import "testing"

func TestPCMGainRampPlaybackChunking(t *testing.T) {
	tests := map[string][]byte{
		"volume-pan-retrigger": {
			0x81, 7, 60, 1, 64, 0,
			0x81, 4, 24, 0,
			0x81, 8, 24, 0, 0,
			0x81, 8, 24, 255, 0,
			0x81, 7, 72, 1, 48, 0,
			0x81, 1, 254, 0,
		},
		"panbrello": {
			0x81, 11, 60, 1, 25, 0xff, 0,
			0x81, 8, 25, 0, 0,
			0x81, 8, 25, 0, 0,
			0x81, 8, 25, 0, 0,
			0x81, 8, 25, 0, 0,
			0x81, 1, 254, 0,
		},
		"volume-slides": {
			0x81, 7, 60, 1, 48, 0,
			0x81, 8, 4, 8, 0,
			0x81, 4, 79, 0,
			0x81, 8, 14, 0xf3, 0,
			0x81, 8, 19, 0x91, 0,
			0x81, 1, 254, 0,
		},
	}
	for name, rows := range tests {
		t.Run(name, func(t *testing.T) {
			testPCMGainRampPlaybackChunking(t, rows)
		})
	}
}

func testPCMGainRampPlaybackChunking(t *testing.T, rows []byte) {
	t.Helper()
	loaded, err := loadIT(testITPitchFixture(rows, 1, true, false))
	if err != nil {
		t.Fatal(err)
	}
	const frames = 8000
	render := func(chunk int) ([]float32, *Tracker) {
		t.Helper()
		w := NewTracker(loaded.Assets, 48000, 1024)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
		out := make([]float32, 2*frames)
		for pos := 0; pos < frames; {
			n := min(chunk, frames-pos)
			if got := w.Read(out[2*pos:2*(pos+n)], n); got != n {
				t.Fatalf("chunk %d at %d: rendered %d frames", chunk, pos, got)
			}
			pos += n
		}
		return out, w
	}
	want, original := render(1024)
	audible := false
	for _, sample := range want {
		audible = audible || sample != 0
	}
	if !audible || original.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("expected audible playback followed by note cut")
	}
	for _, chunk := range []int{1, 17, 127, 960} {
		got, w := render(chunk)
		if w.Clock != original.Clock || w.TrackCursors[0].ActiveVoice != -1 {
			t.Fatalf("chunk %d: clock %+v, voice %d", chunk, w.Clock, w.TrackCursors[0].ActiveVoice)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
			}
		}
	}
}

func TestPCMGainRampShortVoiceReuse(t *testing.T) {
	assets := SongAssets{NumTracks: 1, Samples: []SampleAsset{
		{Data: []float32{1, 1, 1}, Volume: 64, C5Speed: 8363},
		{Data: []float32{1, 1, 1, 1}, Volume: 64, C5Speed: 8363, LoopEnable: true, LoopEnd: 4},
	}}
	w := NewTracker(assets, 48000, 64)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.VoicePitches[voice].StepIncrement = 1
	w.VoiceAmps[voice].TargetLeft = 1
	ResamplerSystem(w, 8)
	out := make([]float32, 16)
	MixerAccumulationSystem(w, out, 8)
	if w.VoiceLives[voice].Active || out[0] != 0 || out[2] <= 0 || out[6] != 0 {
		t.Fatalf("short voice output %v, active %t", out, w.VoiceLives[voice].Active)
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != voice {
		t.Fatal("voice was not reused")
	}
	w.VoicePitches[voice].StepIncrement = 1
	w.VoiceAmps[voice].TargetLeft = 1
	ResamplerSystem(w, 8)
	MixerAccumulationSystem(w, out, 8)
	if out[0] != 0 || out[2] <= 0 {
		t.Fatalf("reused voice retained ramp state: %v", out)
	}
}
