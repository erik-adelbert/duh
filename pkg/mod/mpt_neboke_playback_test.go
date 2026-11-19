package mod

import (
	"math"
	"os"
	"testing"
)

func TestNebokeMPTMEnvelopeFade(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_NEBOKE")
	if path == "" {
		t.Skip("set MPTECS_MPTM_NEBOKE to the Neboke module")
	}
	loaded, err := LoadMPTM(path)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 1024)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 2048)
	var frames int
	var sum float64
	var tailSamples int
	for {
		n := w.Read(buf, 1024)
		if n == 0 {
			break
		}
		for i, sample := range buf[:2*n] {
			if frames+i/2 >= 137*48000 {
				sum += float64(sample) * float64(sample)
				tailSamples++
			}
		}
		frames += n
		if frames > 140*48000 {
			t.Fatal("module did not stop at its expected duration")
		}
	}
	if frames != 6632791 {
		t.Fatalf("rendered %d frames, want 6632791", frames)
	}
	if tailSamples == 0 || math.Sqrt(sum/float64(tailSamples)) > 0.001 {
		t.Fatalf("unexpected ending RMS: %g", math.Sqrt(sum/float64(tailSamples)))
	}
}
