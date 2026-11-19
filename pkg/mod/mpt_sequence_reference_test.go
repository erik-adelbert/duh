package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testMPTSequenceReference(current byte, first, second []mptTestEntry) []byte {
	b := testITPitchFixture(nil, 6, false, false)
	binary.LittleEndian.PutUint16(b[40:], 0x891)
	binary.LittleEndian.PutUint32(b[256+60:], 8363)
	binary.LittleEndian.PutUint16(b[38:], 2)
	binary.LittleEndian.PutUint32(b[198:], 336)
	binary.LittleEndian.PutUint32(b[202:], 420)
	for i, offset := range []int{336, 420} {
		packed := []byte{0x81, 3, byte(60 + i*12), 1, 0, 0, 0, 0}
		binary.LittleEndian.PutUint16(b[offset:], uint16(len(packed)))
		binary.LittleEndian.PutUint16(b[offset+2:], 4)
		copy(b[offset+8:], packed)
	}
	set := mptTestBlock("mptSeqC", []mptTestEntry{
		{"n", []byte{2}}, {"c", []byte{current}},
		{"\x00", mptTestBlock("mptSeq", first)}, {"\x01", mptTestBlock("mptSeq", second)},
	})
	tail := len(b)
	b = append(b, mptTestBlock("mptm", []mptTestEntry{{"mptSeqC", set}})...)
	return binary.LittleEndian.AppendUint32(b, uint32(tail))
}

func renderMPTSequenceReference(t *testing.T, data []byte, args ...string) []float32 {
	t.Helper()
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	path := filepath.Join(t.TempDir(), "sequence.mptm")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	options := []string{"--batch", "--quiet", "--stdout", "--samplerate", "48000", "--channels", "2",
		"--float", "--filter", "2", "--ramping", "0", "--dither", "0"}
	options = append(options, args...)
	options = append(options, path)
	output, err := exec.Command(player, options...).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("openmpt123: %v: %s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	pcm := make([]float32, len(output)/4)
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(output[i*4:]))
	}
	return pcm
}

func TestOpenMPTSequenceDefaults(t *testing.T) {
	first := []mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{0, 0}},
		{"t", binary.LittleEndian.AppendUint32(nil, 1000000)},
		{"s", binary.LittleEndian.AppendUint32(nil, 3)},
	}
	second := []mptTestEntry{
		{"l", []byte{1, 0}}, {"a", []byte{1, 0}},
		{"t", binary.LittleEndian.AppendUint32(nil, 1500000)},
		{"s", binary.LittleEndian.AppendUint32(nil, 6)},
	}
	data := testMPTSequenceReference(1, first, second)
	tail := int(binary.LittleEndian.Uint32(data[len(data)-4:]))
	set, err := parseMPTSequences(data[tail:len(data)-4], 2)
	if err != nil {
		t.Fatal(err)
	}
	if set.current != 1 {
		t.Fatalf("saved editor selection = %d, want 1", set.current)
	}
	for i, want := range []mptSequenceDefaults{{tempo: 1000000, speed: 3}, {tempo: 1500000, speed: 6}} {
		got, err := resolveMPTSequenceDefaults(set.sequences[i], mptSequenceDefaults{tempo: 1250000, speed: 6})
		if err != nil || got != want {
			t.Fatalf("sequence %d defaults = %+v, error %v, want %+v", i, got, err, want)
		}
	}
	path := filepath.Join(t.TempDir(), "sequence.mptm")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	for _, tt := range []struct {
		args     []string
		duration string
	}{
		{[]string{"--info"}, "00:00.700"},
		{[]string{"--info", "--subsong", "0"}, "00:00.300"},
		{[]string{"--info", "--subsong", "1"}, "00:00.400"},
	} {
		out, err := exec.Command(player, append(tt.args, path)...).CombinedOutput()
		if err != nil {
			t.Fatalf("openmpt123: %v: %s", err, out)
		}
		if !strings.Contains(string(out), "Duration...: "+tt.duration) || !strings.Contains(string(out), "Subsongs...: 2") {
			t.Fatalf("unexpected info for %v: %s", tt.args, out)
		}
	}
	for _, tt := range []struct {
		name string
		args []string
		note float64
	}{
		{"default all", nil, 1045.4},
		{"subsong zero", []string{"--subsong", "0"}, 1045.4},
		{"subsong one", []string{"--subsong", "1"}, 2090.8},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := append(tt.args, "--end-time", "0.120")
			pcm := renderMPTSequenceReference(t, data, args...)
			if len(pcm) < 5760*2 {
				t.Fatalf("short reference: %d samples", len(pcm))
			}
			if frequency := referenceTickFrequency(t, pcm, 1); math.Abs(frequency/tt.note-1) > .02 {
				t.Fatalf("frequency %.2f, want %.2f", frequency, tt.note)
			}
		})
	}
}

func TestOpenMPTSequenceRestart(t *testing.T) {
	for _, restart := range []byte{0, 1} {
		t.Run(fmt.Sprintf("restart %d", restart), func(t *testing.T) {
			first := []mptTestEntry{
				{"l", []byte{2, 0}}, {"a", []byte{0, 0, 1, 0}}, {"r", []byte{restart, 0}},
				{"t", binary.LittleEndian.AppendUint32(nil, 1000000)},
				{"s", binary.LittleEndian.AppendUint32(nil, 3)},
			}
			second := []mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}}
			pcm := renderMPTSequenceReference(t, testMPTSequenceReference(1, first, second),
				"--subsong", "0", "--repeat", "1", "--end-time", "0.850")
			if len(pcm) < referenceRate*800/1000*2 {
				t.Fatalf("short reference: %d samples", len(pcm))
			}
			restartedNote := 1045.4
			if restart == 1 {
				restartedNote = 2090.8
			}
			for _, tt := range []struct {
				tick int
				note float64
			}{{2, 1045.4}, {17, 2090.8}, {32, restartedNote}} {
				got := referenceTickFrequency(t, pcm, tt.tick)
				if math.Abs(got/tt.note-1) > .02 {
					t.Fatalf("tick %d frequency %.2f, want %.2f", tt.tick, got, tt.note)
				}
			}
		})
	}
}

func TestOpenMPTSequenceFallbackDefaults(t *testing.T) {
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	for _, tt := range []struct {
		name      string
		overrides []mptTestEntry
		want      string
	}{
		{"absent", nil, "00:00.480"},
		{"zero", []mptTestEntry{{"t", []byte{0, 0, 0, 0}}, {"s", []byte{0, 0, 0, 0}}}, "00:00.480"},
		{"tempo", []mptTestEntry{{"t", binary.LittleEndian.AppendUint32(nil, 1000000)}}, "00:00.600"},
		{"fractional tempo", []mptTestEntry{{"t", binary.LittleEndian.AppendUint32(nil, 1005000)}}, "00:00.597"},
		{"speed", []mptTestEntry{{"s", binary.LittleEndian.AppendUint32(nil, 3)}}, "00:00.240"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			first := append([]mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{0, 0}}}, tt.overrides...)
			second := []mptTestEntry{{"l", []byte{1, 0}}, {"a", []byte{1, 0}}}
			b := testMPTSequenceReference(1, first, second)
			path := filepath.Join(t.TempDir(), "fallback.mptm")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(player, "--info", "--subsong", "0", path).CombinedOutput()
			if err != nil || !strings.Contains(string(out), "Duration...: "+tt.want) {
				t.Fatalf("OpenMPT info: %v\n%s", err, out)
			}
			out, err = exec.Command(player, "--info", "--subsong", "1", path).CombinedOutput()
			if err != nil || !strings.Contains(string(out), "Duration...: 00:00.480") {
				t.Fatalf("second sequence should inherit legacy defaults: %v\n%s", err, out)
			}
			tail := int(binary.LittleEndian.Uint32(b[len(b)-4:]))
			set, err := parseMPTSequences(b[tail:len(b)-4], 2)
			if err != nil {
				t.Fatal(err)
			}
			defaults, err := resolveMPTSequenceDefaults(set.sequences[0], mptSequenceDefaults{tempo: 1250000, speed: 6})
			if err != nil {
				t.Fatal(err)
			}
			duration := float64(defaults.speed) * 4 * 2.5 / (float64(defaults.tempo) / 10000)
			if fmt.Sprintf("%06.3f", duration) != strings.TrimPrefix(tt.want, "00:") {
				t.Fatalf("duration %.3f disagrees with OpenMPT %s", duration, tt.want)
			}
		})
	}
}
