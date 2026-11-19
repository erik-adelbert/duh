package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTMQxxSpecialNotes(t *testing.T) {
	for _, note := range []byte{254, 255} {
		for _, arg := range []byte{0x02, 0x12, 0x92} {
			for _, compatible := range []bool{false, true} {
				t.Run(fmt.Sprintf("note=%d/Q%02X/compatible=%t", note, arg, compatible), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 9, note, 17, arg, 0, 0x81, 8, 17, 0, 0}
					b := mptS70Fixture(t, rows)
					b[50] = 4
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
					mask := byte(1 << (mptITRetrigger % 8))
					b[flag] &^= mask
					if compatible {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					for tick := range 12 {
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

func TestOpenMPTMQxxNoteOffEnvelope(t *testing.T) {
	for _, compatible := range []bool{false, true} {
		for _, arg := range []byte{0x02, 0x92} {
			t.Run(fmt.Sprintf("compatible=%t/Q%02X", compatible, arg), func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 2, 0, 0x81, 9, 255, 17, arg, 0, 0x81, 8, 17, 0, 0}
				b := mptS70Fixture(t, rows)
				b[50] = 4
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				instrument := binary.LittleEndian.Uint32(b[198:])
				setTestEnvelope(b[instrument:instrument+554], 304, 1, []int8{64, 16}, []uint16{0, 20})
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				for tick := range 12 {
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

func TestOpenMPTMQxxSpecialNotesSampleMode(t *testing.T) {
	for _, note := range []byte{254} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("note=%d/compatible=%t", note, compatible), func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 9, note, 17, 0x02, 0, 0x81, 8, 17, 0, 0}
				b := mptTwoPCMSamplesFixture(t, rows, true, true)
				b[50] = 4
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITRetrigger)/8
				mask := byte(1 << (mptITRetrigger % 8))
				b[flag] &^= mask
				if compatible {
					b[flag] |= mask
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				for tick := range 12 {
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

func TestMPTMQxxSpecialValidation(t *testing.T) {
	for _, row := range [][]byte{
		{0x81, 11, 254, 1, 17, 0x02, 0},
		{0x81, 13, 255, 24, 17, 0x02, 0},
	} {
		if _, err := unpackITPatternMode(row, 1, true); err == nil {
			t.Fatalf("unsupported Qxx special-note combination %v accepted", row)
		}
	}
	b := mptTwoPCMSamplesFixture(t, []byte{0x81, 9, 255, 17, 0x02, 0}, true, true)
	if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "note-off in sample mode") {
		t.Fatalf("sample-mode note-off with Qxx: %v", err)
	}
}
