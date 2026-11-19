package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMPitchPanSeparation(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"next notes", []byte{0x81, 3, 72, 2, 0, 0x81, 1, 48, 0, 0x81, 1, 60, 0}},
		{"near center", []byte{0x81, 3, 61, 2, 0, 0x81, 1, 62, 0, 0x81, 1, 60, 0}},
		{"panning effect", []byte{0x81, 3, 72, 2, 0, 0x81, 8, 24, 200, 0, 0x81, 1, 60, 0}},
		{"volume pan", []byte{0x81, 3, 72, 2, 0, 0x81, 4, 176, 0, 0x81, 1, 60, 0}},
		{"pan slide", []byte{0x81, 3, 72, 2, 0, 0x81, 8, 16, 0x20, 0, 0x81, 1, 60, 0}},
		{"portamento", []byte{0x81, 3, 72, 2, 0, 0x81, 9, 48, 7, 4, 0, 0x81, 1, 60, 0}},
		{"same-row panning", []byte{0x81, 15, 72, 2, 176, 24, 200, 0, 0x81, 1, 48, 0, 0x81, 1, 60, 0}},
		{"note off", []byte{0x81, 3, 72, 2, 0, 0x81, 1, 255, 0, 0x81, 3, 60, 2, 0}},
		{"note cut", []byte{0x81, 3, 72, 2, 0, 0x81, 1, 254, 0, 0x81, 3, 60, 2, 0}},
	}
	for _, tt := range tests {
		for _, separation := range []int8{-127, -16, 16, 127} {
			t.Run(fmt.Sprintf("%s/separation=%d", tt.name, separation), func(t *testing.T) {
				b := mptPitchPanProfileFixture(t, true)
				instrument := int(binary.LittleEndian.Uint32(b[198:]))
				b[instrument+22], b[instrument+23] = byte(separation), 60
				b = mptPatternRowsFixture(t, b, tt.rows)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
				audible := 0
				for _, ms := range []int{25, 65, 105} {
					left, right := channelRMS(ref, ms, ms+5, 0), channelRMS(ref, ms, ms+5, 1)
					actualLeft, actualRight := channelRMS(actual, ms, ms+5, 0), channelRMS(actual, ms, ms+5, 1)
					if left+right < .001 {
						continue
					}
					audible++
					want, got := left/(left+right), actualLeft/(actualLeft+actualRight)
					if math.IsNaN(got) || math.Abs(got-want) > .035 {
						t.Errorf("%dms: OpenMPT left fraction %.3f tracker %.3f", ms, want, got)
					}
				}

				if audible < 2 {
					t.Fatalf("only %d audible reference windows", audible)
				}
			})
		}
	}
}

func TestOpenMPTMPitchPanInstrumentDefaults(t *testing.T) {
	for _, restore := range []bool{false, true} {
		for _, reset := range []bool{false, true} {
			for _, samplePan := range []bool{false, true} {
				for _, pan := range []byte{0, 32, 64, 128} {
					for _, mapped := range []bool{false, true} {
						t.Run(fmt.Sprintf("restore=%t/reset=%t/sample=%t/pan=%d/mapped=%t", restore, reset, samplePan, pan, mapped), func(t *testing.T) {
							b := mptPitchPanProfileFixture(t, true)
							instrument := int(binary.LittleEndian.Uint32(b[198:]))
							b[instrument+22], b[instrument+23], b[instrument+25] = 16, 60, pan
							flag := bytesIndex(t, b, ".FSM") + 6
							if restore {
								b[flag+102/8] |= 1 << (102 % 8)
							}
							if !reset {
								b[flag+int(mptITPanningReset)/8] &^= 1 << (mptITPanningReset % 8)
							}
							if samplePan {
								sample := int(binary.LittleEndian.Uint32(b[202:]))
								b[sample+47] = 0x80 | 16
							}
							if mapped {
								b[instrument+64+72*2] = 48
							}
							rows := []byte{0x81, 3, 72, 2, 0, 0x81, 9, 48, 7, 4, 0, 0x81, 1, 60, 0}
							b = mptPatternRowsFixture(t, b, rows)
							ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
							for _, ms := range []int{25, 65, 105} {
								left, right := channelRMS(ref, ms, ms+5, 0), channelRMS(ref, ms, ms+5, 1)
								actualLeft, actualRight := channelRMS(actual, ms, ms+5, 0), channelRMS(actual, ms, ms+5, 1)
								if left+right < .001 || actualLeft+actualRight < .001 {
									t.Fatalf("%dms: silent reference or tracker", ms)
								}
								want, got := left/(left+right), actualLeft/(actualLeft+actualRight)
								if math.Abs(got-want) > .035 {
									t.Errorf("%dms: OpenMPT left fraction %.3f tracker %.3f", ms, want, got)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMPitchPanNNA(t *testing.T) {
	for _, action := range []byte{0, 1, 2, 3} {
		t.Run(fmt.Sprintf("NNA=%d", action), func(t *testing.T) {
			b := mptPitchPanProfileFixture(t, true)
			instrument := int(binary.LittleEndian.Uint32(b[198:]))
			b[instrument+17], b[instrument+22], b[instrument+23] = action, 16, 60
			rows := []byte{0x81, 3, 72, 2, 0, 0x81, 1, 48, 0, 0x81, 8, 24, 200, 0, 0x81, 1, 60, 0}
			b = mptPatternRowsFixture(t, b, rows)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
			for _, ms := range []int{25, 65, 105, 145} {
				left, right := channelRMS(ref, ms, ms+5, 0), channelRMS(ref, ms, ms+5, 1)
				actualLeft, actualRight := channelRMS(actual, ms, ms+5, 0), channelRMS(actual, ms, ms+5, 1)
				if left+right < .001 || actualLeft+actualRight < .001 {
					t.Fatalf("%dms: silent reference or tracker", ms)
				}
				want, got := left/(left+right), actualLeft/(actualLeft+actualRight)
				if math.Abs(got-want) > .05 {
					t.Errorf("%dms: OpenMPT left fraction %.3f tracker %.3f", ms, want, got)
				}
			}
			assertMPTPatternDelayChunking(t, b, referenceRate/8)
		})
	}
}

func TestOpenMPTMPitchPanOPL(t *testing.T) {
	for _, separation := range []int8{-16, 16} {
		t.Run(fmt.Sprintf("separation=%d", separation), func(t *testing.T) {
			b := mptPitchPanProfileFixture(t, true)
			instrument := int(binary.LittleEndian.Uint32(b[194:]))
			b[instrument+22], b[instrument+23], b[instrument+304] = byte(separation), 60, 0
			rows := []byte{0x81, 3, 72, 1, 0, 0x81, 8, 24, 200, 0, 0x81, 1, 60, 0}
			b = mptPatternRowsFixture(t, b, rows)
			ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
			for _, ms := range []int{25, 65, 105} {
				left, right := channelRMS(ref, ms, ms+5, 0), channelRMS(ref, ms, ms+5, 1)
				actualLeft, actualRight := channelRMS(actual, ms, ms+5, 0), channelRMS(actual, ms, ms+5, 1)
				if left+right < .001 || actualLeft+actualRight < .001 {
					t.Fatalf("%dms: silent reference or tracker", ms)
				}
				want, got := left/(left+right), actualLeft/(actualLeft+actualRight)
				if math.Abs(got-want) > .05 {
					t.Errorf("%dms: OpenMPT left fraction %.3f tracker %.3f", ms, want, got)
				}
			}
			assertMPTPatternDelayChunking(t, b, referenceRate/10)
		})
	}
}
