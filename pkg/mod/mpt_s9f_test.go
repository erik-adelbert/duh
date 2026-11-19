package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMS9F(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, loop := range []bool{false, true} {
			for _, note := range []byte{60, 96} {
				t.Run(fmt.Sprintf("mode=%d/loop=%t/note=%d", mode, loop, note), func(t *testing.T) {
					b := mptResamplingFixture(t, mode, note, loop)
					b = mptPatternRowsFixture(t, b, []byte{0x81, 11, note, 2, 19, 0x9f, 0})
					ref, actual := mptSharedEffectRenderForDuration(t, b, 75)
					end := 65
					if note == 96 && !loop {
						end = 10
					}
					assertMPTPCMCorrelation(t, ref, actual, 5, end)
				})
			}
		}
	}
}

func TestOpenMPTMS9FMidNoteAndMemory(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, loop := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/loop=%t", mode, loop), func(t *testing.T) {
				b := mptResamplingFixture(t, mode, 60, loop)
				b = mptPatternRowsFixture(t, b, []byte{
					0x81, 3, 60, 2, 0,
					0x81, 8, 19, 0x9f, 0,
					0x81, 8, 19, 0, 0,
				})
				ref, actual := mptSharedEffectRenderForDuration(t, b, 110)
				assertMPTPCMCorrelation(t, ref, actual, 45, 75)
				assertMPTPCMCorrelation(t, ref, actual, 85, 105)
			})
		}
	}
}

func TestMPTS9FStereoEndAndChunks(t *testing.T) {
	for _, loop := range []bool{false, true} {
		b := mptResamplingFixture(t, 0, 60, loop)
		b = mptPatternRowsFixture(t, b, []byte{0x81, 11, 60, 2, 19, 0x9f, 0})
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		const frames = 8000
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
					t.Fatalf("loop %t chunk %d sample %d: got %g, want %g", loop, chunk, i, got[i], want[i])
				}
			}
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
			w.Read(make([]float32, 512), 256)
			if got := testing.AllocsPerRun(100, func() { ResamplerSystem(w, 256) }); got != 0 {
				t.Fatalf("reverse resampler allocated %g times", got)
			}
		}
	}
	left, right := []float32{0, .1, .2, .3}, []float32{0, -.2, -.4, -.6}
	w := NewTracker(SongAssets{mptNearestResampling: true, Samples: []SampleAsset{{Data: left, RightData: right}}}, 48000, 12)
	w.VoiceLives[0].Active = true
	w.VoiceSamples[0].Position, w.VoiceSamples[0].Reverse = 4-1.0/4294967296.0, true
	w.VoicePitches[0].StepIncrement = .4
	ResamplerSystem(w, 12)
	if !w.VoiceSamples[0].Ended {
		t.Fatal("reverse one-shot did not end")
	}
	for i, value := range w.VoiceSamples[0].ScratchBuf[:12] {
		if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+2*value)) > 1e-6 {
			t.Fatalf("frame %d: right %g, left %g", i, got, value)
		}
	}
}

func TestOpenMPTMS9FPingPong(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			b := mptResamplingFixture(t, mode, 60, true)
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			b[sample+18] |= 0x40
			b = mptPatternRowsFixture(t, b, []byte{0x81, 11, 60, 2, 19, 0x9f, 0})
			ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
			assertMPTPCMCorrelation(t, ref, actual, 5, 90)
		})
	}
}
