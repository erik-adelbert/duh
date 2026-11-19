package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMSEF(t *testing.T) {
	for _, delay := range []byte{0x61, 0x6f, 0xe0, 0xe1, 0xee, 0xef} {
		for _, arg := range []byte{0x01, 0xf1, 0x1f} {
			for _, reuse := range []bool{false, true} {
				t.Run(fmt.Sprintf("S%02X/D%02X/reuse=%t", delay, arg, reuse), func(t *testing.T) {
					rowArg := arg
					if reuse {
						rowArg = 0
					}
					rows := []byte{
						0x82, 15, 60, 2, 48, 4, arg, 0,
						0x81, 8, 19, delay, 0x82, 8, 4, rowArg, 0,
						0x82, 3, 60, 2, 0,
					}
					b := mptS70Fixture(t, rows)
					b[bytesIndex(t, b, "...C")+6] = 2
					ref, actual := mptSharedEffectRenderForDuration(t, b, 720)
					for ms := 45; ms < 690; ms += 20 {
						r, a := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(r-a) > .003 {
							t.Errorf("%dms: OpenMPT %g tracker %g", ms, r, a)
						}
					}
				})
			}
		}
	}
}

func TestMPTMSEFValidation(t *testing.T) {
	for _, rows := range [][]byte{
		{0x81, 9, 254, 19, 0xef, 0},
		{0x81, 8, 19, 0xe2, 0x82, 11, 60, 2, 8, 0x47, 0},
		{0x81, 8, 19, 0xe2, 0x82, 4, 48, 0},
		{0x81, 8, 19, 0xe2, 0x82, 8, 19, 0xd1, 0},
		{0x81, 8, 19, 0xef, 0, 0x81, 8, 19, 0, 0},
	} {
		_, err := loadMPTM(mptS70Fixture(t, rows))
		if err == nil || !strings.Contains(err.Error(), "pattern delay") {
			t.Fatalf("unsupported SEF interaction accepted: %v", err)
		}
	}
}

func TestOpenMPTMPatternDelays(t *testing.T) {
	tests := make([]struct {
		name string
		args []byte
	}, 0, 36)
	for _, family := range []byte{0x60, 0xe0} {
		for x := range byte(16) {
			tests = append(tests, struct {
				name string
				args []byte
			}{fmt.Sprintf("S%02X", family|x), []byte{family | x}})
		}
	}
	for _, args := range [][]byte{{0x62, 0x63}, {0xe0, 0xe2}, {0xe2, 0xe1}, {0xe2, 0x63}} {
		tests = append(tests, struct {
			name string
			args []byte
		}{fmt.Sprintf("S%02X-S%02X", args[0], args[1]), args})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x83, 7, 60, 2, 48, 0}
			for channel, arg := range tt.args {
				rows = append(rows, byte(0x81+channel), 8, 19, arg)
			}
			rows = append(rows, 0, 0x83, 1, 254, 0)
			b := mptS70Fixture(t, rows)
			b[bytesIndex(t, b, "...C")+6] = 3
			ref, actual := mptSharedEffectRenderForDuration(t, b, 2200)
			for ms := 45; ms < 2100; ms += 20 {
				want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
				if math.Abs(got-want) > .003 {
					t.Fatalf("%dms: OpenMPT %g tracker %g", ms, want, got)
				}
			}
		})
	}
}

func TestMPTMPatternDelayChunking(t *testing.T) {
	b := mptS70Fixture(t, []byte{
		0x83, 15, 60, 2, 48, 4, 0x01, 0,
		0x81, 8, 19, 0xe2, 0x82, 8, 19, 0x63, 0x83, 8, 4, 0, 0,
		0x83, 1, 254, 0,
	})
	b[bytesIndex(t, b, "...C")+6] = 3
	want := assertMPTPatternDelayChunking(t, b, referenceRate)
	ref := referenceMPTMForDuration(t, b, 1000)
	for ms := 45; ms < 990; ms += 20 {
		reference, actual := rmsWindow(ref, ms, ms+5), rmsWindow(want, ms, ms+5)
		if math.Abs(actual-reference) > .003 {
			t.Fatalf("%dms: OpenMPT %g tracker %g", ms, reference, actual)
		}
	}
}

func TestMPTMPatternDelayInteractionChunking(t *testing.T) {
	b := mptS70Fixture(t, []byte{
		0x82, 7, 60, 2, 48, 0x85, 7, 64, 2, 48, 0,
		0x81, 8, 19, 0xe2, 0x82, 8, 14, 0x02, 0x83, 8, 23, 0xf2,
		0x84, 8, 20, 0x13, 0x85, 8, 16, 0x02, 0,
		0x82, 1, 254, 0x85, 1, 254, 0,
	})
	b[bytesIndex(t, b, "...C")+6] = 5
	assertMPTPatternDelayChunking(t, b, referenceRate/2)
}

func assertMPTPatternDelayChunking(t *testing.T, b []byte, frames int) []float32 {
	t.Helper()
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	render := func(chunk int) []float32 {
		w := NewTracker(loaded.Assets, referenceRate, 256)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = loaded.InitialSpeed, loaded.InitialTempo, -1
		out := make([]float32, frames*2)
		for frame := 0; frame < len(out)/2; {
			n := min(chunk, len(out)/2-frame)
			if got := w.Read(out[frame*2:(frame+n)*2], n); got != n {
				t.Fatalf("short render at frame %d: %d", frame, got)
			}
			frame += n
		}
		return out
	}
	want := render(256)
	for _, chunk := range []int{1, 17, 127} {
		got := render(chunk)
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("chunk %d sample %d: got %g, want %g", chunk, i, got[i], want[i])
			}
		}
	}
	return want
}

func TestOpenMPTMPatternDelaySlides(t *testing.T) {
	for _, delays := range [][]byte{{0x63}, {0xe2}, {0xee}, {0xe2, 0x63}} {
		for _, speed := range []byte{1, 2} {
			for _, firstTickHandling := range []bool{false, true} {
				for _, effect := range []byte{4, 14, 16, 23} {
					for _, arg := range []byte{0x02, 0x20, 0xf2, 0x2f, 0} {
						t.Run(fmt.Sprintf("%X/speed=%d/firstTick=%t/%c%02X", delays, speed, firstTickHandling, 'A'+effect-1, arg), func(t *testing.T) {
							rows := []byte{0x83, 15, 60, 2, 48, effect, 0x01, 0}
							for channel, delay := range delays {
								rows = append(rows, byte(0x81+channel), 8, 19, delay)
							}
							rows = append(rows, 0x83, 8, effect, arg, 0, 0x83, 1, 254, 0)
							b := mptS70Fixture(t, rows)
							b[bytesIndex(t, b, "...C")+6] = 3
							b[50], b[48] = speed, 64
							b[64+2], b[128+2] = 32, 40
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITFirstTickHandling)/8
							mask := byte(1 << (mptITFirstTickHandling % 8))
							b[flag] &^= mask
							if firstTickHandling {
								b[flag] |= mask
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 720)
							if channelRMS(ref, 15, 16, 0) < .005 {
								t.Fatal("reference fixture is unexpectedly silent")
							}
							for ms := 19; ms < 700; ms += 20 {
								for channel := range 2 {
									want := channelRMS(ref, ms, ms+1, channel)
									got := channelRMS(actual, ms, ms+1, channel)
									if math.Abs(got-want) > .003 {
										t.Errorf("%dms channel %d: OpenMPT %g tracker %g", ms, channel, want, got)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMPatternDelayPitchEffects(t *testing.T) {
	tests := []struct {
		effect, arg byte
	}{
		{5, 0x03}, {5, 0xf3}, {5, 0xe3}, {6, 0x03}, {6, 0xf3},
		{7, 0}, {7, 3}, {8, 0x47}, {8, 0}, {11, 0x02}, {11, 0x2f}, {12, 0x02}, {12, 0xf2},
		{21, 0x47}, {19, 0x10}, {19, 0x11},
	}
	for _, delay := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
		for _, old := range []bool{false, true} {
			for _, firstTick := range []bool{false, true} {
				for _, speed := range []byte{1, 2, 3} {
					for _, tt := range tests {
						t.Run(fmt.Sprintf("%X/old=%t/first=%t/speed=%d/%d-%02x", delay, old, firstTick, speed, tt.effect, tt.arg), func(t *testing.T) {
							flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
							flags &^= 1 << mptMODVBlankTiming
							if !firstTick {
								flags &^= 1 << mptITFirstTickHandling
							}
							if speed == 1 && !firstTick {
								flags &^= 1 << mptSlidesAtSpeed1
							}
							rows := []byte{0x83, 11, 60, 1, 8, 0x47, 0,
								0x83, 9, 72, 7, 8, 0}
							for channel, arg := range delay {
								rows = append(rows, byte(0x81+channel), 8, 19, arg)
							}
							rows = append(rows, 0x83, 8, tt.effect, tt.arg, 0, 0x83, 8, 7, 0, 0)
							b := mptSharedEffectFixture(t, rows, flags)
							b[bytesIndex(t, b, "...C")+6] = 3
							b[50], b[53] = speed, 2
							if old {
								b[44] |= 0x10
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 560)
							for tick := int(2 * speed); tick < 27; tick++ {
								want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
								if math.IsNaN(got) || math.Abs(got/want-1) > .005 {
									t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
								}
								if tt.effect == 11 || tt.effect == 12 {
									ms := tick*20 + 15
									want, got := rmsWindow(ref, ms, ms+4), rmsWindow(actual, ms, ms+4)
									if math.Abs(got-want) > .004 {
										t.Errorf("tick %d: OpenMPT RMS %g tracker %g", tick, want, got)
									}
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMPatternDelayMicrotuning(t *testing.T) {
	for _, delay := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
		for _, silent := range []bool{false, true} {
			for _, arg := range []byte{0, 0x80, 0xc0} {
				t.Run(fmt.Sprintf("%X/silent=%t/%02x", delay, silent, arg), func(t *testing.T) {
					b := mptResamplingFixture(t, 1, 60, true)
					b[53] = 2
					b[bytesIndex(t, b, "...C")+6] = 3
					instrument := int(binary.LittleEndian.Uint32(b[198:]))
					b[instrument+304] = 0
					sample := int(binary.LittleEndian.Uint32(b[202:]))
					data := int(binary.LittleEndian.Uint32(b[sample+72:]))
					for i := range 4096 {
						binary.LittleEndian.PutUint16(b[data+i*2:], uint16(int16(18000*math.Sin(2*math.Pi*float64(i)/32))))
					}
					rows := []byte{0x83, 3, 60, 2, 0}
					if silent {
						rows = append(rows, 0x83, 1, 254, 0)
					}
					for channel, value := range delay {
						rows = append(rows, byte(0x81+channel), 8, 19, value)
					}
					rows = append(rows, 0x83, 8, 30, arg, 0, 0x83, 1, 60, 0)
					b = mptPatternRowsFixture(t, b, rows)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 560)
					start := 2
					if silent {
						frameDelay, rowDelay := 0, 0
						for _, value := range delay {
							if value&0xf0 == 0x60 {
								frameDelay += int(value & 15)
							} else {
								rowDelay = int(value & 15)
							}
						}
						start = 4 + (2+frameDelay)*(1+rowDelay)
					}
					for tick := start; tick < min(start+4, 27); tick++ {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.IsNaN(got) || math.Abs(got/want-1) > .003 {
							t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMPatternDelayLegacyVibrato(t *testing.T) {
	for _, old := range []bool{false, true} {
		for _, effect := range []byte{8, 11, 21} {
			t.Run(fmt.Sprintf("old=%t/effect=%d", old, effect), func(t *testing.T) {
				flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
				flags &^= 1<<mptMODVBlankTiming | 1<<mptITVibratoTremoloPanbrello
				rows := []byte{0x83, 11, 60, 1, 8, 0x47, 0,
					0x81, 8, 19, 0xe2, 0x82, 8, 19, 0x63, 0x83, 8, effect, 0x02, 0}
				b := mptSharedEffectFixture(t, rows, flags)
				b[bytesIndex(t, b, "...C")+6] = 3
				if old {
					b[44] |= 0x10
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 400)
				for tick := 2; tick < 19; tick++ {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.IsNaN(got) || math.Abs(got/want-1) > .005 {
						t.Errorf("tick %d: OpenMPT %.2f Hz tracker %.2f Hz", tick, want, got)
					}
				}
			})
		}
	}
}

func TestOpenMPTMPatternDelayTempo(t *testing.T) {
	for _, delays := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
		for _, mode := range []string{"classic", "modern", "vblank"} {
			for _, arg := range []byte{0x03, 0x14, 0x7d, 0} {
				if mode == "vblank" && arg != 0x03 && arg != 0 {
					continue
				}
				t.Run(fmt.Sprintf("%X/%s/T%02X", delays, mode, arg), func(t *testing.T) {
					rows := []byte{0x83, 11, 60, 2, 20, 0x14, 0}
					for channel, delay := range delays {
						rows = append(rows, byte(0x81+channel), 8, 19, delay)
					}
					rows = append(rows, 0x83, 8, 20, arg, 0, 0x83, 1, 254, 0)
					if mode == "vblank" {
						rows[5] = 3
					}
					b := mptS70Fixture(t, rows)
					b[bytesIndex(t, b, "...C")+6] = 3
					flag := bytesIndex(t, b, ".FSM") + 6
					b[flag] &^= 1 << mptMODVBlankTiming
					switch mode {
					case "modern":
						b[46] |= 4
						b[bytesIndex(t, b, "..MT")+6] = 2
					case "vblank":
						b[flag] |= 1 << mptMODVBlankTiming
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 900)
					lastAudible := func(pcm []float32) int {
						for frame := len(pcm)/2 - 1; frame >= 0; frame-- {
							if math.Abs(float64(pcm[frame*2])) > 1e-6 || math.Abs(float64(pcm[frame*2+1])) > 1e-6 {
								return frame
							}
						}
						return -1
					}
					want, got := lastAudible(ref), lastAudible(actual)
					if want < referenceRate/50 || want >= len(ref)/2-5 || absInt(got-want) > 5 {
						t.Fatalf("cut frame: OpenMPT %d, tracker %d", want, got)
					}
				})
			}
		}
	}
}

func TestOpenMPTMPatternDelayMultipleTempoCommands(t *testing.T) {
	for _, commands := range [][]byte{
		{0x82, 8, 20, 0x03, 0x83, 8, 20, 0x14},
		{0x82, 8, 20, 0x7d, 0x83, 8, 20, 0x14},
	} {
		t.Run(fmt.Sprintf("%X", commands), func(t *testing.T) {
			rows := []byte{0x84, 3, 60, 2, 0, 0x81, 8, 19, 0xe2}
			rows = append(rows, commands...)
			rows = append(rows, 0, 0x84, 1, 254, 0)
			b := mptS70Fixture(t, rows)
			b[bytesIndex(t, b, "...C")+6] = 4
			ref, actual := mptSharedEffectRenderForDuration(t, b, 600)
			for ms := 20; ms < 580; ms++ {
				want, got := rmsWindow(ref, ms, ms+1), rmsWindow(actual, ms, ms+1)
				if math.Abs(got-want) > .003 {
					t.Fatalf("%dms: OpenMPT %g tracker %g", ms, want, got)
				}
			}
		})
	}
}

func TestOpenMPTMPatternDelayGlobalSlideMemory(t *testing.T) {
	for _, perChannel := range []bool{false, true} {
		t.Run(fmt.Sprintf("perChannel=%t", perChannel), func(t *testing.T) {
			b := mptS70Fixture(t, []byte{
				0x84, 7, 60, 2, 48, 0x82, 8, 23, 0x01, 0x83, 8, 23, 0x20, 0,
				0x81, 8, 19, 0xe2, 0x82, 8, 23, 0, 0x83, 8, 23, 0, 0,
				0x84, 1, 254, 0,
			})
			b[bytesIndex(t, b, "...C")+6] = 4
			b[48] = 64
			flag := bytesIndex(t, b, ".FSM") + 6 + int(mptPerChannelGlobalVolSlide)/8
			mask := byte(1 << (mptPerChannelGlobalVolSlide % 8))
			b[flag] &^= mask
			if perChannel {
				b[flag] |= mask
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
			for ms := 19; ms < 280; ms += 20 {
				want, got := rmsWindow(ref, ms, ms+1), rmsWindow(actual, ms, ms+1)
				if math.Abs(got-want) > .003 {
					t.Errorf("%dms: OpenMPT %g tracker %g", ms, want, got)
				}
			}
		})
	}
}

func TestOpenMPTMFadeWithE04(t *testing.T) {
	b := mptResamplingFixture(t, 1, 60, true)
	b = mptPatternRowsFixture(t, b, []byte{0x81, 3, 60, 2, 0, 0x81, 9, 253, 5, 4, 0})
	ref, actual := mptSharedEffectRenderForDuration(t, b, 75)
	var cross, r2, a2 float64
	for i := 45 * 96; i < 70*96; i++ {
		r, a := float64(ref[i]), float64(actual[i])
		cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
	}
	correlation := cross / math.Sqrt(r2*a2)
	if r2 < .001 || a2 < .001 || math.IsNaN(correlation) || correlation < .98 {
		t.Fatalf("correlation %g energy %g/%g", correlation, r2, a2)
	}
}
