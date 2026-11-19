package mod

import (
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var testOPLPatch = oplPatch{0x21, 0x21, 0x3f, 0x00, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0}

func referenceOPL(t *testing.T, patch oplPatch, secondRow []byte) []float32 {
	t.Helper()
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	data := referenceS3M(0, 0, nil)
	data[0x40] = 16
	data[0x100] = 2
	copy(data[0x110:0x11c], patch[:])
	copy(data[0x14c:0x150], "SCRI")
	if len(secondRow) > 0 {
		data[0x31] = 3
		copy(data[0x158:], secondRow)
	}
	path := filepath.Join(t.TempDir(), "opl.s3m")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(player, "--batch", "--quiet", "--stdout", "--samplerate", "48000",
		"--channels", "2", "--float", "--filter", "2", "--ramping", "0",
		"--dither", "0", "--end-time", "0.12", path)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("openmpt123: %v: %s", err, exitErr.Stderr)
		}
		t.Fatal(err)
	}
	const samples = referenceRate * 120 / 1000 * 2
	if len(output) < samples*4 {
		t.Fatalf("reference output has %d bytes, need %d", len(output), samples*4)
	}
	pcm := make([]float32, samples)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(output[i*4:]))
	}
	return pcm
}

func TestOpenMPTOPLSingleNote(t *testing.T) {
	ref := referenceOPL(t, testOPLPatch, nil)
	s := newOPLSynth(referenceRate)
	s.setPatch(0, testOPLPatch)
	s.setVolume(0, 63)
	s.setPanning(0, 128)
	s.setFrequency(0, 261.625, true)
	out := make([]int16, referenceRate*120/1000*2)
	s.render(out)
	actual := make([]float32, len(out))
	for i, v := range out {
		actual[i] = float32(v) / 32768
	}
	for _, tt := range []struct {
		name string
		pcm  []float32
	}{
		{"OpenMPT", ref},
		{"duh OPL3", actual},
	} {
		rms := rmsWindow(tt.pcm, 60, 100)
		if rms < 0.0001 {
			t.Errorf("%s is silent: RMS %g", tt.name, rms)
			continue
		}
		freq := referenceTickFrequency(t, tt.pcm, 4)
		if math.Abs(freq-261.625) > 3 {
			t.Errorf("%s frequency = %.2f Hz, want about 261.625 Hz", tt.name, freq)
		}
	}
}

func TestOpenMPTOPLVolumeChange(t *testing.T) {
	ref := referenceOPL(t, testOPLPatch, []byte{0x40, 32, 0})
	s := newOPLSynth(referenceRate)
	s.setPatch(0, testOPLPatch)
	s.setVolume(0, 63)
	s.setPanning(0, 128)
	s.setFrequency(0, 261.625, true)
	out := make([]int16, referenceRate*120/1000*2)
	boundary := referenceRate * 60 / 1000 * 2
	s.render(out[:boundary])
	s.setVolume(0, 32)
	s.render(out[boundary:])
	actual := make([]float32, len(out))
	for i, v := range out {
		actual[i] = float32(v) / 32768
	}
	beforeRef, beforeActual := rmsWindow(ref, 30, 50), rmsWindow(actual, 30, 50)
	if beforeRef < 1e-5 || beforeActual < 1e-5 {
		t.Fatal("silent comparison baseline")
	}
	want := rmsWindow(ref, 95, 115) / beforeRef
	got := rmsWindow(actual, 95, 115) / beforeActual
	if math.Abs(want-got) > .05 {
		t.Fatalf("relative volume after change: OpenMPT %.3f, duh OPL3 %.3f", want, got)
	}
}

func TestOpenMPTOPLNoteCut(t *testing.T) {
	ref := referenceOPL(t, testOPLPatch, []byte{0x20, 254, 0, 0})
	s := newOPLSynth(referenceRate)
	s.setPatch(0, testOPLPatch)
	s.setVolume(0, 63)
	s.setPanning(0, 128)
	s.setFrequency(0, 261.625, true)
	out := make([]int16, referenceRate*120/1000*2)
	boundary := referenceRate * 60 / 1000 * 2
	s.render(out[:boundary])
	s.noteOff(0)
	s.setVolume(0, 0)
	s.render(out[boundary:])
	actual := make([]float32, len(out))
	for i, v := range out {
		actual[i] = float32(v) / 32768
	}
	beforeRef, beforeActual := rmsWindow(ref, 30, 50), rmsWindow(actual, 30, 50)
	if beforeRef < 1e-5 || beforeActual < 1e-5 {
		t.Fatal("silent comparison baseline")
	}
	want := rmsWindow(ref, 95, 115) / beforeRef
	got := rmsWindow(actual, 95, 115) / beforeActual
	if math.Abs(want-got) > .02 {
		t.Fatalf("relative volume after cut: OpenMPT %.3f, duh OPL3 %.3f", want, got)
	}
}

func TestOPLLevel(t *testing.T) {
	for _, tt := range []struct {
		volume, level, want uint8
	}{
		{0, 0x80, 0xbf},
		{32, 0x80, 0x9f},
		{63, 0x80, 0x80},
		{0, 0xbf, 0xbf},
	} {
		if got := oplLevel(tt.volume, tt.level); got != tt.want {
			t.Errorf("volume %d, level %#x: got %#x, want %#x", tt.volume, tt.level, got, tt.want)
		}
	}
}

func TestOPLPanningAndVolume(t *testing.T) {
	render := func(pan int, volume uint8) (float64, float64) {
		s := newOPLSynth(referenceRate)
		s.setPatch(0, testOPLPatch)
		s.setVolume(0, volume)
		s.setPanning(0, pan)
		s.setFrequency(0, 261.625, true)
		out := make([]int16, 4800*2)
		s.render(out)
		var left, right float64
		for i := 2400; i < len(out); i += 2 {
			left += float64(out[i]) * float64(out[i])
			right += float64(out[i+1]) * float64(out[i+1])
		}
		return math.Sqrt(left / 3600), math.Sqrt(right / 3600)
	}
	left, leftRight := render(0, 63)
	rightLeft, right := render(256, 63)
	centerLeft, centerRight := render(128, 63)
	quiet, _ := render(128, 32)
	muted, _ := render(128, 0)
	if left < 1 || right < 1 || leftRight != 0 || rightLeft != 0 {
		t.Errorf("hard panning: left (%g, %g), right (%g, %g)", left, leftRight, rightLeft, right)
	}
	if centerLeft < 1 || centerRight < 1 || math.Abs(centerLeft-centerRight) > 1 {
		t.Errorf("center panning: (%g, %g)", centerLeft, centerRight)
	}
	if !(muted < quiet && quiet < centerLeft) {
		t.Errorf("volume levels: zero %g, half %g, full %g", muted, quiet, centerLeft)
	}
}

func TestOPLRenderNoAllocations(t *testing.T) {
	s := newOPLSynth(referenceRate)
	s.setPatch(0, testOPLPatch)
	s.setVolume(0, 63)
	s.setPanning(0, 128)
	s.setFrequency(0, 261.625, true)
	buf := make([]int16, 512)
	s.render(buf)
	if got := testing.AllocsPerRun(100, func() { s.render(buf) }); got != 0 {
		t.Fatalf("OPL render allocated %g times per call", got)
	}
}

func TestOPLRenderChunkingAndIndependence(t *testing.T) {
	setup := func() *oplSynth {
		s := newOPLSynth(referenceRate)
		s.setPatch(9, testOPLPatch)
		s.setVolume(9, 63)
		s.setPanning(9, 128)
		s.setFrequency(9, 261.625, true)
		return s
	}
	whole, chunks := setup(), setup()
	a, b := make([]int16, 4096), make([]int16, 4096)
	whole.render(a)
	for i := 0; i < len(b); i += 128 {
		chunks.render(b[i : i+128])
	}
	nonzero := false
	for i := range a {
		nonzero = nonzero || a[i] != 0
		if a[i] != b[i] {
			t.Fatalf("sample %d: whole %d, chunks %d", i, a[i], b[i])
		}
	}
	if !nonzero {
		t.Fatal("second OPL bank produced silence")
	}
	whole.noteOff(9)
	c, d := make([]int16, 512), make([]int16, 512)
	whole.render(c)
	chunks.render(d)
	for i := range c {
		if c[i] != d[i] {
			return
		}
	}
	t.Fatal("note-off did not affect just its own synthesizer")
}
