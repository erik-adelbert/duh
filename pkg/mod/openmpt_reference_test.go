package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/erik-adelbert/duh/pkg/mod/internal/testfixtures"
)

const referenceRate = 48000

// referenceS3M creates a single-channel S3M with a sample and one effect.
func referenceS3M(effect, argument byte, sample []byte) []byte {
	return testfixtures.S3M(effect, argument, sample)
}

func referencePCM(t *testing.T, data []byte) (reference, actual []float32) {
	return referencePCMWithInterpolation(t, data, "1")
}

func referencePCMWithInterpolation(t *testing.T, data []byte, interpolation string) (reference, actual []float32) {
	return referencePCMForDuration(t, data, interpolation, 120)
}

func referencePCMForDuration(t *testing.T, data []byte, interpolation string, milliseconds int) (reference, actual []float32) {
	return referencePCMWithChunks(t, data, interpolation, milliseconds, 0)
}

func referencePCMWithChunks(t *testing.T, data []byte, interpolation string, milliseconds, chunkSize int) (reference, actual []float32) {
	t.Helper()
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	extension := ".s3m"
	loader := LoadS3M
	if len(data) >= 4 && string(data[:4]) == "IMPM" {
		extension = ".it"
		loader = LoadIT
	}
	path := filepath.Join(t.TempDir(), "reference"+extension)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(player, "--batch", "--quiet", "--stdout", "--samplerate", "48000",
		"--channels", "2", "--float", "--filter", interpolation, "--ramping", "0",
		"--dither", "0", "--end-time", fmt.Sprintf("%.3f", float64(milliseconds)/1000), path)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("openmpt123: %v: %s", err, exitErr.Stderr)
		}
		t.Fatalf("openmpt123: %v", err)
	}
	frames := referenceRate * milliseconds / 1000
	if len(output) < frames*8 {
		t.Fatalf("OpenMPT returned %d bytes, need %d", len(output), frames*8)
	}
	reference = make([]float32, frames*2)
	for i := range reference {
		reference[i] = math.Float32frombits(binary.LittleEndian.Uint32(output[i*4:]))
	}
	loaded, err := loader(path)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, frames)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	actual = make([]float32, frames*2)
	if chunkSize <= 0 {
		chunkSize = frames
	}
	for offset := 0; offset < frames; {
		n := min(chunkSize, frames-offset)
		if got := w.Read(actual[offset*2:], n); got != n {
			t.Fatalf("rendered %d frames at offset %d, want %d", got, offset, n)
		}
		offset += n
	}
	return reference, actual
}

func rmsWindow(pcm []float32, startMS, endMS int) float64 {
	var sum float64
	first, end := startMS*referenceRate/1000*2, endMS*referenceRate/1000*2
	for _, v := range pcm[first:end] {
		sum += float64(v) * float64(v)
	}
	return math.Sqrt(sum / float64(end-first))
}

func TestOpenMPTNoteDelay(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 224
	}
	ref, actual := referencePCM(t, referenceS3M(19, 0xd3, sample))
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		t.Run(name, func(t *testing.T) {
			before, after := rmsWindow(pcm, 35, 55), rmsWindow(pcm, 70, 90)
			if before > 0.00001 || after < 0.005 {
				t.Fatalf("SD3: before = %g, after = %g", before, after)
			}
		})
	}
}

func TestOpenMPTSampleOffset(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 128
		if i >= 256 {
			sample[i] = 224
		}
	}
	ref, actual := referencePCM(t, referenceS3M(15, 1, sample))
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		t.Run(name, func(t *testing.T) {
			if level := rmsWindow(pcm, 5, 15); level < 0.005 {
				t.Fatalf("O01 did not start at offset 256: RMS = %g", level)
			}
		})
	}
}

func TestOpenMPTRetrigger(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 128
		if i < 100 {
			sample[i] = 224
		}
	}
	ref, actual := referencePCM(t, referenceS3M(17, 2, sample))
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		t.Run(name, func(t *testing.T) {
			for _, ms := range []int{0, 40, 80} {
				level := rmsWindow(pcm, ms+3, ms+8)
				if level < 0.005 {
					t.Errorf("Q02 missing retrigger near %d ms: RMS = %g", ms, level)
				}

			}
			if level := rmsWindow(pcm, 25, 35); level > 0.0001 {
				t.Errorf("Q02 expected silence between triggers: RMS = %g", level)
			}

		})
	}
}

func TestOpenMPTVolumeSlide(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 192
	}
	ref, actual := referencePCM(t, referenceS3M(4, 0x08, sample))
	refBaseline, actualBaseline := referencePCM(t, referenceS3M(0, 0, sample))
	for _, ms := range []int{35, 55, 75, 95, 115} {
		baseRef := rmsWindow(refBaseline, ms, ms+4)
		baseActual := rmsWindow(actualBaseline, ms, ms+4)
		if baseRef < 0.001 || baseActual < 0.001 {
			t.Fatalf("silent baseline at %d ms: OpenMPT %g, tracker %g", ms, baseRef, baseActual)
		}
		want := rmsWindow(ref, ms, ms+4) / baseRef
		got := rmsWindow(actual, ms, ms+4) / baseActual
		if math.Abs(got-want) > 0.03 {
			t.Errorf("D08 at %d ms: normalized tracker RMS = %.3f, OpenMPT = %.3f", ms, got, want)
		}
	}
}
