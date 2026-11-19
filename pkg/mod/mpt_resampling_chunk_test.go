package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestMPTSincChunkingAndAllocation(t *testing.T) {
	b := mptResamplingFixture(t, 3, 96, true)
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	const frames = 8000
	render := func(chunk int) []float32 {
		t.Helper()
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
		t.Fatalf("sinc resampler allocated %g times", got)
	}
}

func TestMPTSincPingPongLoaderRejection(t *testing.T) {
	b := mptResamplingFixture(t, 3, 96, true)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x40
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "sinc resampling") {
		t.Fatalf("ping-pong sinc loop: %v", err)
	}

}

func TestMPTSincStereo(t *testing.T) {
	instruments := make([]ITInstrument, 1)
	if err := applyMPTResampling(instruments, []uint32{3}); err != nil {
		t.Fatal(err)
	}
	left, right := make([]float32, 64), make([]float32, 64)
	for i := range left {
		left[i] = float32(math.Sin(float64(i)))
		right[i] = -left[i] * .5
	}
	w := NewTracker(SongAssets{IT: true, ITInstruments: instruments,
		Samples: []SampleAsset{{Data: left, RightData: right, LoopEnable: true, LoopStart: 8, LoopEnd: 64}}}, 48000, 32)
	w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 1}
	w.VoiceSamples[0].SampleID = 0
	w.VoicePitches[0].StepIncrement = 1.7
	ResamplerSystem(w, 32)
	audible := false
	for i, value := range w.VoiceSamples[0].ScratchBuf[:32] {
		audible = audible || value != 0
		if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+value*.5)) > 1e-6 {
			t.Fatalf("frame %d: left %g, right %g", i, value, got)
		}
	}
	if !audible {
		t.Fatal("sinc stereo test was silent")
	}
}
