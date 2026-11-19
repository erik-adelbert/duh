package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func mptSustainLoopFixture(t *testing.T, mode byte, release byte) []byte {
	t.Helper()
	b := mptResamplingFixture(t, mode, 60, false)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x20
	binary.LittleEndian.PutUint32(b[sample+64:], 128)
	binary.LittleEndian.PutUint32(b[sample+68:], 193)
	pos := int(binary.LittleEndian.Uint32(b[sample+72:]))
	for i := range 4096 {
		freq := .05
		if i >= 193 {
			freq = .27
		}
		binary.LittleEndian.PutUint16(b[pos+2*i:], uint16(int16(18000*math.Sin(float64(i)*2*math.Pi*freq))))
	}
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[instrument:instrument+554], 304, 1, []int8{64}, []uint16{0})
	b[instrument+17] = 1
	if release == 2 {
		b[instrument+18], b[instrument+19] = 3, 1
	}
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	length := int(binary.LittleEndian.Uint16(b[pattern:]))
	clear(b[pattern+8 : pattern+8+length])
	binary.LittleEndian.PutUint16(b[pattern+2:], 32)
	rows := []byte{0x81, 3, 60, 2, 0, 0, 0}
	switch release {
	case 1:
		rows = append(rows, 0x81, 1, 255, 0)
	case 2, 3:
		rows = append(rows, 0x81, 3, 64, 2, 0)
	default:
		rows = append(rows, 0)
	}
	copy(b[pattern+8:], rows)
	return b
}

func TestMPTSustainLoopValidation(t *testing.T) {
	b := mptSustainLoopFixture(t, 1, 1)
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	s := loaded.Assets.Samples[0]
	if !s.mptSustainLoop || s.mptSustainStart != 128 || s.mptSustainEnd != 193 {
		t.Fatalf("sustain loop: %+v", s)
	}
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	for _, tt := range []struct {
		name string
		edit func([]byte)
		want string
	}{
		{"empty", func(b []byte) { binary.LittleEndian.PutUint32(b[sample+68:], 128) }, "invalid IT sustain loop"},
		{"out of bounds", func(b []byte) { binary.LittleEndian.PutUint32(b[sample+68:], 4097) }, "invalid IT sustain loop"},
		{"sample mode", func(b []byte) { b[44] &^= 4 }, "unsupported encoding or loop"},
		{"old version", func(b []byte) { binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000) }, "pre-1.26"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			copyData := append([]byte(nil), b...)
			tt.edit(copyData)
			if _, err := loadMPTM(copyData); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestOpenMPTMSustainLoop(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, release := range []byte{0, 1, 2} {
			t.Run(fmt.Sprintf("mode=%d/release=%d", mode, release), func(t *testing.T) {
				ref, actual := mptSharedEffectRenderForDuration(t, mptSustainLoopFixture(t, mode, release), 220)
				for _, win := range [][2]int{{50, 90}, {145, 190}} {
					var cross, r2, a2 float64
					for i := win[0] * 96; i < win[1]*96; i++ {
						r, a := float64(ref[i]), float64(actual[i])
						cross += r * a
						r2 += r * r
						a2 += a * a
					}
					correlation := cross / math.Sqrt(r2*a2)
					if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
						t.Fatalf("%d-%d ms: correlation %.4f, energy %.4f/%.4f", win[0], win[1], correlation, r2, a2)
					}
				}
			})
		}
	}
}

func TestOpenMPTMSustainLoopReleaseBoundaries(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, end := range []uint32{129, 130, 131, 150, 191, 192, 194, 195, 196, 198, 200} {
			t.Run(fmt.Sprintf("mode=%d/end=%d", mode, end), func(t *testing.T) {
				b := mptSustainLoopFixture(t, mode, 1)
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				binary.LittleEndian.PutUint32(b[sample+68:], end)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 180)
				var cross, r2, a2 float64
				for i := 130 * 96; i < 170*96; i++ {
					r, a := float64(ref[i]), float64(actual[i])
					cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
				}
				correlation := cross / math.Sqrt(r2*a2)
				if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
					t.Fatalf("release correlation %.4f, energy %.4f/%.4f", correlation, r2, a2)
				}
			})
		}
	}
}

func TestOpenMPTMSustainDuplicateReleaseEffect(t *testing.T) {
	for _, mode := range []byte{1, 3} {
		t.Run(fmt.Sprintf("mode=%d", mode), func(t *testing.T) {
			ref, actual := mptSharedEffectRenderForDuration(t, mptSustainLoopFixture(t, mode, 2), 200)
			baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptSustainLoopFixture(t, mode, 3), 200)
			var cross, r2, a2 float64
			for i := 145 * 96; i < 190*96; i++ {
				r, a := float64(ref[i]-baseRef[i]), float64(actual[i]-baseActual[i])
				cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
			}
			correlation := cross / math.Sqrt(r2*a2)
			if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .95 {
				t.Fatalf("duplicate-release difference correlation %.4f, energy %.4f/%.4f", correlation, r2, a2)
			}
		})
	}
}

func TestMPTSustainChunkingAndAllocation(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, release := range []byte{1, 2} {
			t.Run(fmt.Sprintf("mode=%d/release=%d", mode, release), func(t *testing.T) {
				loaded, err := loadMPTM(mptSustainLoopFixture(t, mode, release))
				if err != nil {
					t.Fatal(err)
				}
				const frames = 10000
				render := func(chunk int) []float32 {
					w := NewTracker(loaded.Assets, referenceRate, 1024)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
					out := make([]float32, frames*2)
					for pos := 0; pos < frames; {
						n := min(chunk, frames-pos)
						if got := w.Read(out[pos*2:(pos+n)*2], n); got != n {
							t.Fatalf("short render at frame %d: %d", pos, got)
						}
						pos += n
					}
					return out
				}
				want := render(1024)
				for _, chunk := range []int{1, 17, 127, 960} {
					got := render(chunk)
					for i := range want {
						if got[i] != want[i] {
							t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
						}
					}
				}
				w := NewTracker(loaded.Assets, referenceRate, 256)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
				buf := make([]float32, 512)
				w.Read(buf, 256)
				if got := testing.AllocsPerRun(100, func() { ResamplerSystem(w, 256) }); got != 0 {
					t.Fatalf("sustain resampler allocated %g times", got)
				}
			})
		}
	}
}

func TestMPTSustainStereoRelease(t *testing.T) {
	left, right := make([]float32, 256), make([]float32, 256)
	for i := range left {
		left[i] = float32(math.Sin(float64(i)))
		right[i] = -.5 * left[i]
	}

	for _, sinc := range []bool{false, true} {
		instruments := []ITInstrument{{mptSincResampling: sinc}}
		w := NewTracker(SongAssets{IT: true, ITInstruments: instruments,
			Samples: []SampleAsset{{Data: left, RightData: right, mptSustainLoop: true, mptSustainStart: 8, mptSustainEnd: 32}}}, 48000, 64)
		w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 1}
		w.VoiceSamples[0].SampleID = 0
		w.VoicePitches[0].StepIncrement = 1
		ResamplerSystem(w, 64)
		if !w.VoiceSamples[0].mptLooped {
			t.Fatal("sustain loop did not wrap")
		}
		releaseITVoice(w, 0)
		if w.VoiceSamples[0].mptLooped || !w.VoiceLives[0].KeyOff {
			t.Fatal("release did not clear sustain history")
		}
		ResamplerSystem(w, 64)
		if w.VoiceSamples[0].Position <= 32 {
			t.Fatal("released voice did not leave the sustain loop")
		}
		for i, value := range w.VoiceSamples[0].ScratchBuf[:64] {
			if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+.5*value)) > 1e-6 {
				t.Fatalf("sinc=%t frame %d: left %g, right %g", sinc, i, value, got)
			}
		}
	}
}
