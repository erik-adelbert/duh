package mod

import (
	"math"
	"os"
	"testing"
)

func loadITPlaybackFixture(t *testing.T, env, name string) *S3MLoadResult {
	t.Helper()
	path := os.Getenv(env)
	if path == "" {
		t.Skipf("set %s to the %s fixture", env, name)
	}
	loaded, err := LoadIT(path)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func checkITPlaybackSmoke(t *testing.T, loaded *S3MLoadResult) {
	t.Helper()
	w := NewTracker(loaded.Assets, referenceRate, 1024)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	out := make([]float32, 1024*2)
	var energy float64
	for frames := 0; frames < 5*referenceRate; {
		n := w.Read(out, min(1024, 5*referenceRate-frames))
		if n == 0 {
			t.Fatal("song ended before 5 seconds")
		}
		for _, sample := range out[:n*2] {
			if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
				t.Fatal("non-finite sample")
			}
			energy += float64(sample) * float64(sample)
		}
		frames += n
	}
	if energy == 0 {
		t.Fatal("rendered song is silent")
	}
}
