package mod

import (
	"math"
	"testing"
)

func TestMPTHighPassStereoChunkingAndAllocation(t *testing.T) {
	left, right := make([]float32, 96), make([]float32, 96)
	for i := range left {
		left[i] = float32(math.Sin(float64(i) * .37))
		right[i] = left[i] * -.5
	}
	assets := SongAssets{Samples: []SampleAsset{{Data: left, RightData: right, LoopEnable: true, LoopStart: 8, LoopEnd: 96}}}
	makeWorld := func() *Tracker {
		w := NewTracker(assets, 48000, 256)
		w.VoiceLives[0].Active = true
		w.VoicePitches[0].StepIncrement = .8
		w.VoiceFilters[0] = newITFilter(70, 60, 48000, true, false, false)
		setITFilterMode(&w.VoiceFilters[0], true)
		w.VoiceRightFilters[0] = w.VoiceFilters[0]
		return w
	}
	whole, split := makeWorld(), makeWorld()
	ResamplerSystem(whole, 256)
	for start := 0; start < 256; {
		n := min(17, 256-start)
		ResamplerSystem(split, n)
		for i := range n {
			l, r := split.VoiceSamples[0].ScratchBuf[i], split.VoiceSamples[0].RightScratchBuf[i]
			if l != whole.VoiceSamples[0].ScratchBuf[start+i] || r != whole.VoiceSamples[0].RightScratchBuf[start+i] ||
				math.Abs(float64(r+l*.5)) > 1e-6 {
				t.Fatalf("frame %d: split %g/%g, whole %g/%g", start+i, l, r,
					whole.VoiceSamples[0].ScratchBuf[start+i], whole.VoiceSamples[0].RightScratchBuf[start+i])
			}
		}
		start += n
	}
	if got := testing.AllocsPerRun(100, func() { ResamplerSystem(split, 256) }); got != 0 {
		t.Fatalf("high-pass resampler allocated %g times", got)
	}
}
