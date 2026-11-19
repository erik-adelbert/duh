package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func stereoITFixture(bits int, compressed, it215 bool, pan byte) ([]byte, []int, []int) {
	left, right := make([]int, 4096), make([]int, 4096)
	for i := range left {
		left[i] = int(math.Sin(float64(i)*2*math.Pi*500/48000)*float64(int(1)<<uint(bits-2))) & ((1 << uint(bits)) - 1)
		right[i] = int(math.Sin(float64(i)*2*math.Pi*750/48000)*float64(int(1)<<uint(bits-2))) & ((1 << uint(bits)) - 1)
	}
	b := testIT()[:512]
	b[64] = pan
	b[256+18] = 1 | 4
	if bits == 16 {
		b[256+18] |= 2
	}
	if compressed {
		b[256+18] |= 8
		if it215 {
			b[256+46] |= 4
		}
		b = append(b, testITCompressed(left, bits, it215)...)
		b = append(b, testITCompressed(right, bits, it215)...)
	} else {
		for _, channel := range [][]int{left, right} {
			for _, value := range channel {
				if bits == 16 {
					b = binary.LittleEndian.AppendUint16(b, uint16(value))
				} else {
					b = append(b, byte(value))
				}
			}
		}
	}
	return b, left, right
}

func TestLoadITStereo(t *testing.T) {
	for _, bits := range []int{8, 16} {
		for _, compression := range []string{"plain", "214", "215"} {
			t.Run(fmt.Sprintf("%d-%s", bits, compression), func(t *testing.T) {
				b, left, right := stereoITFixture(bits, compression != "plain", compression == "215", 32)
				loaded, err := loadIT(b)
				if err != nil {
					t.Fatal(err)
				}
				sample := loaded.Assets.Samples[0]
				for channel, values := range [][]int{left, right} {
					data := sample.Data
					if channel == 1 {
						data = sample.RightData
					}
					if len(data) != len(values) {
						t.Fatalf("channel %d: length %d", channel, len(data))
					}
					for i, value := range values {
						if value >= 1<<uint(bits-1) {
							value -= 1 << uint(bits)
						}
						want := float32(value) / float32(int(1)<<uint(bits-1))
						if data[i] != want {
							t.Fatalf("channel %d sample %d = %g, want %g", channel, i, data[i], want)
						}
					}
				}
				if _, err := loadIT(b[:len(b)-1]); err == nil {
					t.Fatal("accepted truncated right channel")
				}
			})
		}
	}
	b, _, _ := stereoITFixture(8, false, false, 32)
	binary.LittleEndian.PutUint16(b[40:], 0x213)
	if _, err := loadIT(b); err == nil {
		t.Fatal("accepted ambiguous old stereo flag")
	}
}

func TestOpenMPTITStereoPlayback(t *testing.T) {
	for _, pan := range []byte{0, 16, 32, 48, 64} {
		t.Run(fmt.Sprintf("pan-%d", pan), func(t *testing.T) {
			b, left, right := stereoITFixture(16, true, true, pan)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			for channel, samples := range [][]int{left, right} {
				mono := testIT()[:512]
				mono[64] = pan
				for _, value := range samples {
					mono = binary.LittleEndian.AppendUint16(mono, uint16(value))
				}
				refMono, actualMono := referencePCMWithInterpolation(t, mono, "2")
				var power float64
				for i := channel; i < len(ref); i += 2 {
					power += float64(refMono[i]) * float64(refMono[i])
					if math.Abs(float64(ref[i]-refMono[i])) > 1e-6 || actual[i] != actualMono[i] {
						t.Fatalf("channel %d sample %d: OpenMPT stereo/mono %g/%g, tracker %g/%g",
							channel, i/2, ref[i], refMono[i], actual[i], actualMono[i])
					}
				}
				if (pan != 0 || channel != 1) && (pan != 64 || channel != 0) && power == 0 {
					t.Fatalf("channel %d unexpectedly silent", channel)
				}
			}
		})
	}
}

func TestStereoResamplingAndFilters(t *testing.T) {
	for _, loop := range []string{"none", "forward", "pingpong"} {
		t.Run(loop, func(t *testing.T) {
			sample := SampleAsset{
				Data: []float32{0.025, 0.05, 0.075, 0.1}, RightData: []float32{-0.05, -0.1, -0.15, -0.2},
				C5Speed: 8363, Volume: 64, LoopStart: 1, LoopEnd: 4,
				LoopEnable: loop != "none", LoopPingPong: loop == "pingpong",
			}
			w := NewTracker(SongAssets{NumTracks: 1, Samples: []SampleAsset{sample}}, 48000, 16)
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			voice := w.TrackCursors[0].ActiveVoice
			w.VoicePitches[voice].StepIncrement = 0.5
			w.VoiceFilters[voice] = newITFilter(80, 90, 48000, true, false, false)
			w.VoiceRightFilters[voice] = w.VoiceFilters[voice]
			ResamplerSystem(w, 16)
			for i, left := range w.VoiceSamples[voice].ScratchBuf {
				right := w.VoiceSamples[voice].RightScratchBuf[i]
				if math.Abs(float64(right+2*left)) > 1e-5 {
					t.Fatalf("frame %d: left %g right %g", i, left, right)
				}
			}
		})
	}
}
