package mod

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMDelayCut(t *testing.T) {
	for _, speed := range []byte{2, 3, 6} {
		for _, x := range []byte{0, 1, 2, 6} {
			for _, y := range []byte{0, 1, 2, 6} {
				for _, stopsSample := range []bool{false, true} {
					for _, withNote := range []bool{false, true} {
						t.Run(fmt.Sprintf("speed=%d/x=%d/y=%d/cut=%t/note=%t", speed, x, y, stopsSample, withNote), func(t *testing.T) {
							rows := []byte{0x81, 3, 60, 2, 0}
							if withNote {
								rows = append(rows, 0x81, 11, 60, 2, 29, x<<4|y, 0)
							} else {
								rows = append(rows, 0x81, 8, 29, x<<4|y, 0)
							}

							rows = append(rows, 0, 0)
							b := mptS70Fixture(t, rows)
							b[50] = speed
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITSCxStopsSample)/8
							mask := byte(1 << (mptITSCxStopsSample % 8))
							b[flag] &^= mask
							if stopsSample {
								b[flag] |= mask
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 400)
							for tick := range 19 {
								ms := tick*20 + 15
								want, got := rmsWindow(ref, ms, ms+3), rmsWindow(actual, ms, ms+3)
								if math.Abs(got-want) > .005 {
									t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
								}
							}
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTMDelayCutInstrumentOnly(t *testing.T) {
	for _, arg := range []byte{0x01, 0x11, 0x22, 0x31} {
		for _, flag := range []bool{false, true} {
			for _, instr := range []byte{1, 2} {
				t.Run(fmt.Sprintf("arg=%02X/flag=%t/instrument=%d", arg, flag, instr), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 10, instr, 29, arg, 0, 0, 0}
					b := mptInstrumentWithoutNoteFixture(t, rows, flag, 0)
					b[50] = 3
					ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
					for tick := range 14 {
						ms := tick*20 + 15
						want, got := rmsWindow(ref, ms, ms+3), rmsWindow(actual, ms, ms+3)
						if math.Abs(got-want) > .005 {
							t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMDelayCutSampleMode(t *testing.T) {
	for _, arg := range []byte{0x00, 0x01, 0x11, 0x22, 0x31} {
		for _, cut := range []bool{false, true} {
			for _, volume := range []bool{false, true} {
				t.Run(fmt.Sprintf("arg=%02X/cut=%t/volume=%t", arg, cut, volume), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 1, 0}
					if volume {
						rows = append(rows, 0x81, 15, 60, 1, 24, 29, arg, 0)
					} else {
						rows = append(rows, 0x81, 11, 60, 1, 29, arg, 0)
					}
					rows = append(rows, 0, 0)
					b := mptTwoPCMSamplesFixture(t, rows, true, true)
					b[50] = 3
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITSCxStopsSample)/8
					mask := byte(1 << (mptITSCxStopsSample % 8))
					b[flag] &^= mask
					if cut {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
					for tick := range 14 {
						ms := tick*20 + 15
						want, got := rmsWindow(ref, ms, ms+3), rmsWindow(actual, ms, ms+3)
						if math.Abs(got-want) > .005 {
							t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestMPTMDelayCutValidation(t *testing.T) {
	for _, tt := range []struct {
		row  []byte
		want string
	}{
		{[]byte{0x81, 9, 254, 29, 0x11, 0}, "special note"},
		{[]byte{0x81, 12, 85, 29, 0x11, 0}, "volume effect"},
		{[]byte{0x81, 10, 1, 29, 0x00, 0}, "instrument-only"},
		{[]byte{0x81, 10, 1, 29, 0x10, 0}, "instrument-only"},
		{[]byte{0x81, 10, 1, 29, 0x20, 0}, "instrument-only"},
	} {
		if _, err := unpackITPatternMode(tt.row, 1, true); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("unsupported delay-cut combination %v: %v", tt.row, err)
		}
	}
	if _, err := loadMPTM(mptS70Fixture(t, []byte{0x81, 8, 29, 0x11, 0x82, 8, 19, 0xe2, 0})); err == nil {
		t.Fatal("delay-cut with pattern delay accepted")
	}
}
