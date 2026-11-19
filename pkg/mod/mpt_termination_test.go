package mod

import (
	"math"
	"os"
	"testing"
)

func TestMPTPlaybackTermination(t *testing.T) {
	tests := []struct {
		name string
		rows [][]ModCommand
		want int
	}{
		{"self jump", [][]ModCommand{{{EffectCmd: 0x0b, EffectArg: 0}}}, 1},
		{"backward jump", [][]ModCommand{{{}}, {{}}, {{EffectCmd: 0x0b, EffectArg: 0}}}, 3},
		{"finite loop", [][]ModCommand{{{EffectCmd: 0x15, EffectArg: 0xb0}}, {{}}, {{EffectCmd: 0x15, EffectArg: 0xb2}}, {{}}}, 10},
		{"finite loop then jump", [][]ModCommand{{{EffectCmd: 0x15, EffectArg: 0xb0}}, {{EffectCmd: 0x15, EffectArg: 0xb2}}, {{EffectCmd: 0x0b, EffectArg: 0}}}, 7},
		{"natural end", [][]ModCommand{{{}}, {{}}}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := mptMixProfile{}
			w := NewTracker(SongAssets{
				IT: true, NumTracks: 1, OrderList: []int{0},
				Patterns:        []PatternAsset{{Rows: tt.rows}},
				ITCompatibility: ITCompatibility{mptMixProfile: &profile},
			}, referenceRate, 1024)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			buf := make([]float32, 1024*2)
			total := 0
			for range 20 {
				n := w.Read(buf, 1024)
				total += n
				if n == 0 {
					break
				}
			}
			if want := tt.want * 960; !w.Clock.SongFinished || total != want {
				t.Fatalf("finished=%v frames=%d, want %d", w.Clock.SongFinished, total, want)
			}
		})
	}
}

func TestHeartOfFireTerminates(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_HEART_OF_FIRE")
	if path == "" {
		t.Skip("set MPTECS_MPTM_HEART_OF_FIRE to the 486 Heart of Fire fixture")
	}
	loaded, err := LoadMPTM(path)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 4096*2)
	frames := 0
	for frames < 240*referenceRate {
		n := w.Read(buf, 4096)
		frames += n
		if n == 0 {
			break
		}
	}
	duration := float64(frames) / referenceRate
	if !w.Clock.SongFinished || math.Abs(duration-234.265) > 1 {
		t.Fatalf("finished=%v duration=%.3fs, OpenMPT duration 234.265s", w.Clock.SongFinished, duration)
	}
}
