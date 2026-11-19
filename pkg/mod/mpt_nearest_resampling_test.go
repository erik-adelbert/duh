package mod

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTNearestPingPongLoading(t *testing.T) {
	b := mptSongResamplingFixture(t, 0, 5, true)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x40
	if _, err := loadMPTM(b); err != nil {
		t.Fatalf("nearest ping-pong: %v", err)
	}
	b[44] &^= 4
	if _, err := loadMPTM(b); err != nil {
		t.Fatalf("sample-mode nearest ping-pong: %v", err)
	}
}

func TestOpenMPTMNearestPitchesAndShortLoop(t *testing.T) {
	for _, note := range []byte{60, 96, 108} {
		t.Run(fmt.Sprint(note), func(t *testing.T) {
			b := mptResamplingFixture(t, 0, note, true)
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			binary.LittleEndian.PutUint32(b[sample+56:], 193)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
			assertMPTPCMCorrelation(t, ref, actual, 30, 80)
		})
	}
}

func mptSampleModeResamplingFixture(t *testing.T, mode byte) []byte {
	t.Helper()
	b := mptSongResamplingFixture(t, mode, 5, true)
	b[44] &^= 4
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	songProperties := bytes.Index(b[bounds.propertiesStart:bounds.serializedStart], []byte("STPM"))
	if songProperties < 0 {
		t.Fatal("song properties not found")
	}
	data := append([]byte(nil), b[:bounds.propertiesStart]...)
	data = append(data, b[bounds.propertiesStart+songProperties:bounds.serializedStart]...)
	tail := len(data)
	data = append(data, b[bounds.serializedStart:bounds.serializedEnd]...)
	b = binary.LittleEndian.AppendUint32(data, uint32(tail))
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	b[pattern+11] = 1
	return b
}

func TestOpenMPTMSampleModeResampling(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			b := mptSampleModeResamplingFixture(t, mode)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
			assertMPTPCMCorrelation(t, ref, actual, 10, 50)
		})
	}
}

func TestMPTNearestChunkingAndAllocation(t *testing.T) {
	loaded, err := loadMPTM(mptSongResamplingFixture(t, 0, 5, true))
	if err != nil {
		t.Fatal(err)
	}
	assertMPTNearestChunkingAndAllocation(t, loaded)
}

func assertMPTNearestChunkingAndAllocation(t *testing.T, loaded *S3MLoadResult) {
	t.Helper()
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
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
			}
		}
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
	w.Read(make([]float32, 512), 256)
	if got := testing.AllocsPerRun(100, func() { ResamplerSystem(w, 256) }); got != 0 {
		t.Fatalf("nearest resampler allocated %g times", got)
	}
}

func TestMPTNearestStereoAndEnd(t *testing.T) {
	left := []float32{0, .1, .2, .3}
	right := []float32{0, -.2, -.4, -.6}
	w := NewTracker(SongAssets{mptNearestResampling: true, Samples: []SampleAsset{{Data: left, RightData: right}}}, 48000, 12)
	w.VoiceLives[0].Active = true
	w.VoiceSamples[0].SampleID = 0
	w.VoicePitches[0].StepIncrement = .4
	ResamplerSystem(w, 12)
	want := []float32{0, 0, .1, .1, .2, .2, .2, .3, .3, .3, .3, 0}
	for i, value := range want {
		if got := w.VoiceSamples[0].ScratchBuf[i]; got != value {
			t.Fatalf("frame %d: got %g, want %g", i, got, value)
		}
		if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+2*value)) > 1e-6 {
			t.Fatalf("frame %d: right %g, left %g", i, got, value)
		}
	}
}
