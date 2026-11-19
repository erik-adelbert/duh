package mod

import "testing"

func TestTickTiming(t *testing.T) {
	tests := []struct {
		name       string
		rate       float64
		tempo      float64
		tempoCmd   uint8
		wantFrames int
	}{
		{"default tempo", 48000, 125, 0, 960},
		{"different sample rate", 44100, 125, 0, 882},
		{"tempo change on row", 48000, 125, 150, 800},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([][]ModCommand, 2)
			for i := range rows {
				rows[i] = make([]ModCommand, 1)
			}
			if tt.tempoCmd != 0 {
				rows[0][0] = ModCommand{EffectCmd: 0x14, EffectArg: tt.tempoCmd}
			}
			w := NewTracker(SongAssets{
				OrderList: []int{0},
				Patterns:  []PatternAsset{{Rows: rows}},
				NumTracks: 1,
			}, tt.rate, 64)
			w.Clock.Tempo = tt.tempo
			w.Clock.Tick = -1

			if !TickSchedulerSystem(w) {
				t.Fatal("first tick did not start")
			}
			PatternFetchSystem(w)
			GlobalFlowSystem(w)
			if w.Clock.SamplesPerTick != tt.wantFrames || w.Clock.SamplesRemaining != tt.wantFrames {
				t.Fatalf("tick duration = %d, remaining = %d, want %d", w.Clock.SamplesPerTick, w.Clock.SamplesRemaining, tt.wantFrames)
			}

			w.Clock.SamplesRemaining = 0
			if !TickSchedulerSystem(w) {
				t.Fatal("second tick did not start")
			}
			if w.Clock.SamplesPerTick != tt.wantFrames {
				t.Fatalf("next tick duration = %d, want %d", w.Clock.SamplesPerTick, tt.wantFrames)
			}
		})
	}
}

func TestReadAdvancesAtTrackerTempo(t *testing.T) {
	rows := make([][]ModCommand, 2)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	w := NewTracker(SongAssets{
		OrderList: []int{0},
		Patterns:  []PatternAsset{{Rows: rows}},
		NumTracks: 1,
	}, 48000, 960)
	w.Clock.Tick = -1

	out := make([]float32, 960*2)
	for tick := range 6 {
		if got := w.Read(out, 960); got != 960 {
			t.Fatalf("tick %d: rendered %d frames, want 960", tick, got)
		}
		if w.Clock.Row != 0 || w.Clock.Tick != tick {
			t.Fatalf("tick %d: row = %d, tick = %d", tick, w.Clock.Row, w.Clock.Tick)
		}
	}
	if got := w.Read(out, 1); got != 1 || w.Clock.Row != 1 || w.Clock.Tick != 0 {
		t.Fatalf("next row: frames = %d, row = %d, tick = %d", got, w.Clock.Row, w.Clock.Tick)
	}
}
