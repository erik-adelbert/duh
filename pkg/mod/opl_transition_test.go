package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func transitionFixture(firstOPL, porta, compatible bool) ([]byte, SongAssets) {
	b := testITPitchFixture(nil, 3, true, false)
	binary.LittleEndian.PutUint16(b[40:], 0x891)
	binary.LittleEndian.PutUint16(b[36:], 2)
	if compatible {
		b[44] |= 0x20
	}
	binary.LittleEndian.PutUint32(b[198:], uint32(len(b)))
	binary.LittleEndian.PutUint32(b[202:], 336)
	header := append([]byte(nil), b[256:336]...)
	header[18], header[46] = 1, 0x40
	binary.LittleEndian.PutUint32(header[48:], 12)
	binary.LittleEndian.PutUint32(header[60:], 8363)
	binary.LittleEndian.PutUint32(header[72:], uint32(len(b)+len(header)))
	b = append(b, header...)
	b = append(b, testOPLPatch[:]...)
	binary.LittleEndian.PutUint32(b[256+60:], 8363)
	for i := range 4096 {
		value := int16(math.Sin(float64(i)*2*math.Pi/16) * 16000)
		binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
	}
	a, z := byte(1), byte(2)
	if firstOPL {
		a, z = z, a
	}
	rows := []byte{0x81, 3, 60, a, 0, 0x81, 3, 72, z, 0}
	if porta {
		rows = []byte{0x81, 3, 60, a, 0, 0x81, 11, 72, z, 7, 16, 0}
	}
	rows = append(rows, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(rows)))
	copy(b[344:], rows)
	offset := len(b)
	b = append(b, []byte("228mptm")...)
	b = binary.LittleEndian.AppendUint32(b, uint32(offset))

	pcm := make([]float32, 4096)
	for i := range pcm {
		pcm[i] = float32(int16(binary.LittleEndian.Uint16(b[512+i*2:]))) / 32768
	}
	assets := SongAssets{IT: true, NumTracks: 1, ITGlobalVolume: 128, ITMixGain: 1,
		ITChannelVolume: [MaxTracks]uint8{64}, ITCompatibility: ITCompatibility{LinearSlides: true, CompatibleGxx: compatible, oplPortaSampleRetrigger: true, oplInstrumentSwitch: true},
		Samples: []SampleAsset{
			{Data: pcm, LoopEnable: true, LoopEnd: 3840, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64},
			{opl: &testOPLPatch, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64},
		}}
	return b, assets
}

func referenceTransition(t *testing.T, data []byte) []float32 {
	return referenceMPTMForDuration(t, data, 120)
}

func referenceMPTMForDuration(t *testing.T, data []byte, milliseconds int) []float32 {
	t.Helper()
	player, err := exec.LookPath("openmpt123")
	if err != nil {
		t.Skip("openmpt123 is not installed")
	}
	path := filepath.Join(t.TempDir(), "transition.mptm")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(player, "--batch", "--quiet", "--stdout", "--samplerate", "48000",
		"--channels", "2", "--float", "--filter", "2", "--ramping", "0", "--dither", "0",
		"--end-time", fmt.Sprintf("%.3f", float64(milliseconds)/1000), path)
	output, err := cmd.Output()
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
	frames := referenceRate * milliseconds / 1000
	if len(pcm) < frames*2 {
		t.Fatalf("short reference: %d samples", len(pcm))
	}
	return pcm[:frames*2]
}

func transitionTone(pcm []float32, hz float64) float64 {
	var real, imaginary float64
	for frame := 3840; frame < 5280; frame++ {
		angle := 2 * math.Pi * hz * float64(frame) / 48000
		real += float64(pcm[frame*2]) * math.Cos(angle)
		imaginary += float64(pcm[frame*2]) * math.Sin(angle)
	}
	return math.Hypot(real, imaginary) / 1440
}

func TestOpenMPTOPLPCMTransitions(t *testing.T) {
	for _, firstOPL := range []bool{false, true} {
		for _, porta := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("opl=%v/porta=%v/compat=%v", firstOPL, porta, compatible), func(t *testing.T) {
					b, assets := transitionFixture(firstOPL, porta, compatible)
					ref := referenceTransition(t, b)
					first, second := uint8(1), uint8(2)
					if firstOPL {
						first, second = second, first
					}
					rows := make([][]ModCommand, 64)
					for i := range rows {
						rows[i] = make([]ModCommand, 1)
					}
					rows[0][0] = ModCommand{Note: 61, Instrument: first}
					rows[1][0] = ModCommand{Note: 73, Instrument: second}
					if porta {
						rows[1][0].EffectCmd, rows[1][0].EffectArg = 3, 16
					}
					assets.OrderList, assets.Patterns = []int{0}, []PatternAsset{{Rows: rows}}
					w := NewTracker(assets, referenceRate, 5760)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
					actual := make([]float32, 5760*2)
					if n := w.Read(actual, 5760); n != 5760 {
						t.Fatalf("short render: %d", n)
					}
					for _, tt := range []struct {
						pcm []float32
						hz  float64
					}{{ref, map[bool]float64{false: 522.7, true: 261.7}[firstOPL]},
						{actual, map[bool]float64{false: 522.7, true: 261.7}[firstOPL]}} {
						if got := referenceTickFrequency(t, tt.pcm, 1); math.Abs(got/tt.hz-1) > .01 {
							t.Fatalf("initial frequency %.2f, want %.2f", got, tt.hz)
						}
					}
					want := 523.4
					if firstOPL {
						want = 1045.4
					}
					if got := referenceTickFrequency(t, actual, 4); math.Abs(got/want-1) > .01 {
						t.Fatalf("after transition frequency %.2f, want %.2f", got, want)
					}
					// The reference leaves the old OPL note sounding on this transition.
					if !firstOPL || !porta {
						if got := referenceTickFrequency(t, ref, 4); math.Abs(got/want-1) > .01 {
							t.Fatalf("OpenMPT frequency %.2f, want %.2f", got, want)
						}
					} else if transitionTone(ref, want) < .01 || transitionTone(ref, 261.7) < .01 {
						t.Fatal("OpenMPT reference did not contain both old OPL and new PCM notes")
					}
					voice := w.TrackCursors[0].ActiveVoice
					if voice < 0 || w.VoiceSamples[voice].SampleID != int(second)-1 ||
						(w.VoiceSamples[voice].oplChannel != 0) != !firstOPL {
						t.Fatal("wrong foreground voice after transition")
					}
					if firstOPL && w.oplVoices[0] != -1 {
						t.Fatal("old OPL voice was not released")
					}
				})
			}
		}
	}
}
