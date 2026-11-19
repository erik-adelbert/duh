package mod

import (
	"math"
	"os"
	"strings"
	"testing"
)

func TestRingtoneMPTMPlayback(t *testing.T) {
	path := os.Getenv("MPTECS_MPTM_RINGTONE")
	if path == "" {
		t.Skip("set MPTECS_MPTM_RINGTONE to the Ringtone Chaos fixture")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := loadMPTM(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Warnings) == 0 || !strings.Contains(loaded.Warnings[0], "plug-ins are bypassed") {
		t.Fatalf("missing plug-in bypass warning: %v", loaded.Warnings)
	}
	w := NewTracker(loaded.Assets, referenceRate, 4096)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	buf := make([]float32, 4096*2)
	first := make([]float32, 5*referenceRate*2)
	frames := 0
	var energy float64
	for frames < 190*referenceRate {
		n := w.Read(buf, 4096)
		if frames < 5*referenceRate {
			copy(first[frames*2:], buf[:n*2])
		}
		for _, sample := range buf[:n*2] {
			if math.IsNaN(float64(sample)) || math.IsInf(float64(sample), 0) {
				t.Fatal("non-finite sample")
			}
			energy += float64(sample) * float64(sample)
		}
		frames += n
		if n == 0 {
			break
		}
	}
	if !w.Clock.SongFinished {
		t.Fatalf("song did not terminate after %.2f seconds", float64(frames)/referenceRate)
	}
	if energy == 0 {
		t.Fatal("rendered song is silent")
	}
	duration := float64(frames) / referenceRate
	if math.Abs(duration-179.199) > 1 {
		t.Fatalf("duration %.3fs, OpenMPT duration 179.199s", duration)
	}
	ref := renderMPTSequenceReference(t, data, "--end-time", "5")
	for second := range 5 {
		start, end := second*1000, (second+1)*1000
		got, want := rmsWindow(first, start, end), rmsWindow(ref, start, end)
		t.Logf("second %d RMS: tracker %.4f, OpenMPT %.4f", second, got, want)
	}
}
