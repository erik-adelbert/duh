package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMNearestPingPong(t *testing.T) {
	for _, version := range []uint32{0x01250000, 0x01280100, 0x01300053, 0x01300054, 0x01321200} {
		for _, pingPongMode := range []bool{false, true} {
			for _, note := range []byte{48, 60, 84, 96, 108} {
				for _, end := range []uint32{130, 131, 135, 193, 2048} {
					t.Run(fmt.Sprintf("%08x/mode=%t/note=%d/end=%d", version, pingPongMode, note, end), func(t *testing.T) {
						b := mptResamplingFixture(t, 0, note, true)
						sample := int(binary.LittleEndian.Uint32(b[202:]))
						b[sample+18] |= 0x40
						binary.LittleEndian.PutUint32(b[sample+56:], end)
						binary.LittleEndian.PutUint32(b[sample+60:], 16384)
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
						flags := bytesIndex(t, b, ".FSM") + 6
						b[flags+4] &^= 2
						if pingPongMode {
							b[flags+4] |= 2
						}
						ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
						assertMPTPCMCorrelation(t, ref, actual, 10, 90)
					})
				}
			}
		}
	}
}

func TestMPTNearestPingPongChunkingAndAllocation(t *testing.T) {
	b := mptSongResamplingFixture(t, 0, 5, true)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x40
	binary.LittleEndian.PutUint32(b[sample+56:], 135)
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	assertMPTNearestChunkingAndAllocation(t, loaded)
}

func TestOpenMPTMNearestPingPongSampleMode(t *testing.T) {
	b := mptSampleModeResamplingFixture(t, 0)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x40
	binary.LittleEndian.PutUint32(b[sample+56:], 193)
	binary.LittleEndian.PutUint32(b[sample+60:], 16384)
	ref, actual := mptSharedEffectRenderForDuration(t, b, 100)
	assertMPTPCMCorrelation(t, ref, actual, 10, 90)
}

func TestMPTNearestPingPongStereo(t *testing.T) {
	left := []float32{0, .1, .2, .3, .4}
	right := []float32{0, -.2, -.4, -.6, -.8}
	assets := SongAssets{mptNearestResampling: true, Samples: []SampleAsset{{
		Data: left, RightData: right, LoopEnable: true, LoopPingPong: true, LoopStart: 1, LoopEnd: 5,
	}}}
	w := NewTracker(assets, 48000, 32)
	w.VoiceLives[0].Active = true
	w.VoicePitches[0].StepIncrement = .4
	ResamplerSystem(w, 32)
	for i, value := range w.VoiceSamples[0].ScratchBuf[:32] {
		if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+2*value)) > 1e-6 {
			t.Fatalf("frame %d: right %g, left %g", i, got, value)
		}
	}
}
