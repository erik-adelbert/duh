package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTSustainAndNormalLoop(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, pingPong := range []bool{false, true} {
			if mode == 3 && pingPong {
				continue
			}
			for _, loop := range [][2]uint32{{32, 96}, {100, 160}, {160, 240}, {240, 320}, {0, 193}} {
				for _, release := range []byte{0, 1, 2} {
					t.Run(fmt.Sprintf("mode=%d/pingpong=%t/loop=%v/release=%d", mode, pingPong, loop, release), func(t *testing.T) {
						b := mptSustainLoopFixture(t, mode, release)
						sample := int(binary.LittleEndian.Uint32(b[202:]))
						b[sample+18] |= 0x10
						if pingPong {
							b[sample+18] |= 0x40
						}
						binary.LittleEndian.PutUint32(b[sample+52:], loop[0])
						binary.LittleEndian.PutUint32(b[sample+56:], loop[1])
						ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
						for _, win := range [][2]int{{50, 90}, {145, 190}} {
							assertMPTPCMCorrelation(t, ref, actual, win[0], win[1])
						}
					})
				}
			}
		}
	}
}

func TestOpenMPTSustainNormalLoopReverseRelease(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		for _, pingPong := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/pingpong=%t", mode, pingPong), func(t *testing.T) {
				b := mptSustainLoopFixture(t, mode, 1)
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				b[sample+18] |= 0x10
				if pingPong {
					b[sample+18] |= 0x40
				}
				binary.LittleEndian.PutUint32(b[sample+52:], 100)
				binary.LittleEndian.PutUint32(b[sample+56:], 160)
				b = mptPatternRowsFixture(t, b, []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, 0x9f, 0, 0, 0x81, 1, 255, 0})
				ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
				assertMPTPCMCorrelation(t, ref, actual, 145, 190)
			})
		}
	}
}

func TestOpenMPTSustainNormalLoopOffset(t *testing.T) {
	for _, mode := range []byte{0, 1, 3} {
		for _, offset := range []byte{1, 2} {
			t.Run(fmt.Sprintf("mode=%d/offset=%d", mode, offset), func(t *testing.T) {
				b := mptSustainLoopFixture(t, mode, 1)
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				b[sample+18] |= 0x10
				binary.LittleEndian.PutUint32(b[sample+52:], 64)
				binary.LittleEndian.PutUint32(b[sample+56:], 160)
				binary.LittleEndian.PutUint32(b[sample+68:], 500)
				b = mptPatternRowsFixture(t, b, []byte{0x81, 11, 60, 2, 15, offset, 0, 0, 0, 0x81, 1, 255, 0})
				ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
				assertMPTPCMCorrelation(t, ref, actual, 5, 90)
				assertMPTPCMCorrelation(t, ref, actual, 145, 190)
			})
		}
	}
}

func TestMPTSustainNormalLoopReleaseState(t *testing.T) {
	for _, pingPong := range []bool{false, true} {
		b := mptSustainLoopFixture(t, 1, 1)
		sample := int(binary.LittleEndian.Uint32(b[202:]))
		b[sample+18] |= 0x10
		if pingPong {
			b[sample+18] |= 0x40
		}
		binary.LittleEndian.PutUint32(b[sample+52:], 100)
		binary.LittleEndian.PutUint32(b[sample+56:], 160)
		assertMPTPatternDelayChunking(t, b, 9000)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, tt := range []struct{ position, want float64 }{{150.75, 150.75}, {160.75, 160.75}, {161.75, 101}, {180.75, 120}} {
			w := NewTracker(loaded.Assets, referenceRate, 256)
			w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 2}
			w.VoiceSamples[0] = SamplePlayback{SampleID: 0, Position: tt.position, Reverse: true, mptLooped: true}
			releaseITVoice(w, 0)
			pb := &w.VoiceSamples[0]
			if pb.Position != tt.want || pb.Reverse != pingPong || pb.mptLooped || !w.VoiceLives[0].KeyOff {
				t.Fatalf("pingpong=%t position=%g: release position=%g reverse=%t looped=%t keyoff=%t", pingPong, tt.position, pb.Position, pb.Reverse, pb.mptLooped, w.VoiceLives[0].KeyOff)
			}
		}
	}
}

func TestMPTSustainNormalLoopStereoRelease(t *testing.T) {
	left, right := make([]float32, 256), make([]float32, 256)
	for i := range left {
		left[i] = float32(math.Sin(float64(i)))
		right[i] = -.5 * left[i]
	}
	for _, pingPong := range []bool{false, true} {
		w := NewTracker(SongAssets{IT: true, ITInstruments: []ITInstrument{{}},
			Samples: []SampleAsset{{Data: left, RightData: right, LoopEnable: true, LoopPingPong: pingPong,
				LoopStart: 8, LoopEnd: 24, mptSustainLoop: true, mptSustainStart: 32, mptSustainEnd: 64}}}, 48000, 64)
		w.VoiceLives[0] = VoiceLifecycle{Active: true, ITInstrument: 1}
		w.VoiceSamples[0].SampleID = 0
		w.VoicePitches[0].StepIncrement = .75
		ResamplerSystem(w, 64)
		releaseITVoice(w, 0)
		ResamplerSystem(w, 64)
		for i, value := range w.VoiceSamples[0].ScratchBuf[:64] {
			if got := w.VoiceSamples[0].RightScratchBuf[i]; math.Abs(float64(got+.5*value)) > 1e-6 {
				t.Fatalf("pingpong=%t frame %d: left %g, right %g", pingPong, i, value, got)
			}
		}
	}
}
