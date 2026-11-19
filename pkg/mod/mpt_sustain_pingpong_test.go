package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMSustainPingPong(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		directions := [2]bool{}
		for _, loop := range []struct {
			flags      byte
			start, end uint32
		}{{0, 0, 0}, {0x10, 32, 96}, {0x10, 100, 160}, {0x50, 100, 160}, {0x50, 160, 240}, {0x50, 240, 320}} {
			for _, end := range []uint32{130, 150, 193, 198} {
				t.Run(fmt.Sprintf("mode=%d/normal=%02X-%d-%d/end=%d", mode, loop.flags, loop.start, loop.end, end), func(t *testing.T) {
					b := mptSustainLoopFixture(t, mode, 1)
					sample := int(binary.LittleEndian.Uint32(b[202:]))
					b[sample+18] |= 0x80 | loop.flags
					binary.LittleEndian.PutUint32(b[sample+68:], end)
					if loop.flags != 0 {
						binary.LittleEndian.PutUint32(b[sample+52:], loop.start)
						binary.LittleEndian.PutUint32(b[sample+56:], loop.end)
					}
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}
					w := NewTracker(loaded.Assets, referenceRate, 1920)
					w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
					if got := w.Read(make([]float32, 1920*2), 1920); got != 1920 || w.VoiceLives[0].KeyOff {
						t.Fatal("voice was not held before the release row")
					}
					if w.VoiceSamples[0].Reverse {
						directions[1] = true
					} else {
						directions[0] = true
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
					for _, win := range [][2]int{{5, 35}, {41, 70}, {130, 180}} {
						assertMPTPCMCorrelation(t, ref, actual, win[0], win[1])
					}
					assertMPTPatternDelayChunking(t, b, referenceRate/5)
				})
			}
		}
		if !directions[0] || !directions[1] {
			t.Errorf("mode %d did not cover release in both directions: %v", mode, directions)
		}
	}
}

func TestOpenMPTMSustainPingPongStereo(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		t.Run(fmt.Sprintf("mode=%d", mode), func(t *testing.T) {
			b := mptSustainLoopFixture(t, mode, 1)
			sample := int(binary.LittleEndian.Uint32(b[202:]))
			data := int(binary.LittleEndian.Uint32(b[sample+72:]))
			bounds, err := locateMPTExtensions(b)
			if err != nil {
				t.Fatal(err)
			}
			pcm := append([]byte(nil), b[data:data+8192]...)
			for i := range 4096 {
				v := int16(14000 * math.Sin(float64(i)*.07))
				pcm = binary.LittleEndian.AppendUint16(pcm, uint16(v))
			}
			b[sample+18] |= 0x84
			binary.LittleEndian.PutUint32(b[sample+72:], uint32(bounds.propertiesStart))
			out := append([]byte(nil), b[:bounds.propertiesStart]...)
			out = append(out, pcm...)
			out = append(out, b[bounds.propertiesStart:]...)
			binary.LittleEndian.PutUint32(out[len(out)-4:], uint32(bounds.serializedStart+len(pcm)))
			ref, actual := mptSharedEffectRenderForDuration(t, out, 220)
			for _, channel := range []int{0, 1} {
				for _, win := range [][2]int{{5, 35}, {41, 70}, {130, 180}} {
					var cross, r2, a2 float64
					for i := win[0] * 48; i < win[1]*48; i++ {
						r, a := float64(ref[i*2+channel]), float64(actual[i*2+channel])
						cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
					}
					if corr := cross / math.Sqrt(r2*a2); r2 < .001 || a2 < .001 || math.IsNaN(corr) || corr < .98 {
						t.Fatalf("channel %d, %d-%d ms: correlation %.4f, energy %.4f/%.4f", channel, win[0], win[1], corr, r2, a2)
					}
				}
			}
			assertMPTPatternDelayChunking(t, out, 10000)
		})
	}
}

func TestOpenMPTMSustainPingPongOffsetAndProfile(t *testing.T) {
	for _, mode := range []byte{0, 1} {
		for _, version := range []uint32{0x01260100, 0x01280100, 0x01300100} {
			for _, offset := range []byte{0, 1, 2} {
				for _, pingPongMode := range []bool{false, true} {
					t.Run(fmt.Sprintf("mode=%d/version=%x/offset=%d/pingpong=%t", mode, version, offset, pingPongMode), func(t *testing.T) {
						b := mptSustainLoopFixture(t, mode, 1)
						sample := int(binary.LittleEndian.Uint32(b[202:]))
						b[sample+18] |= 0x80 | 0x10
						binary.LittleEndian.PutUint32(b[sample+52:], 64)
						binary.LittleEndian.PutUint32(b[sample+56:], 160)
						binary.LittleEndian.PutUint32(b[sample+68:], 500)
						binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
						flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITPingPongMode)/8
						if pingPongMode {
							b[flag] |= 1 << (uint(mptITPingPongMode) % 8)
						} else {
							b[flag] &^= 1 << (uint(mptITPingPongMode) % 8)
						}
						if offset != 0 {
							b = mptPatternRowsFixture(t, b, []byte{0x81, 11, 60, 2, 15, offset, 0, 0, 0, 0x81, 1, 255, 0})
						}
						if mode == 1 && !pingPongMode {
							if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "non-IT ping-pong mode") {
								t.Fatalf("unsupported linear ping-pong profile: %v", err)
							}
							return
						}
						ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
						assertMPTPCMCorrelation(t, ref, actual, 5, 35)
						assertMPTPCMCorrelation(t, ref, actual, 145, 190)
						assertMPTPatternDelayChunking(t, b, 10000)
					})
				}
			}
		}
	}
}

func TestMPTSustainPingPongValidation(t *testing.T) {
	for _, mode := range []byte{0, 1, 2, 3} {
		b := mptSustainLoopFixture(t, mode, 1)
		sample := int(binary.LittleEndian.Uint32(b[202:]))
		b[sample+18] |= 0x80
		loaded, err := loadMPTM(b)
		if mode >= 2 {
			if err == nil || !strings.Contains(err.Error(), "unsupported ping-pong loop") {
				t.Errorf("mode %d: %v", mode, err)
			}
		} else if err != nil {
			t.Errorf("mode %d: %v", mode, err)
		} else if !loaded.Assets.Samples[0].itSustainPingPong {
			t.Errorf("mode %d: missing ping-pong sustain flag", mode)
		}
	}
	b := mptSustainLoopFixture(t, 1, 1)
	sample := int(binary.LittleEndian.Uint32(b[202:]))
	b[sample+18] |= 0x80
	binary.LittleEndian.PutUint32(b[sample+68:], 129)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "one-sample ping-pong sustain loop") {
		t.Fatalf("one-sample loop: %v", err)
	}
}
