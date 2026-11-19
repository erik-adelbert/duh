package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMS7NewNoteActions(t *testing.T) {
	for arg := byte(0x73); arg <= 0x76; arg++ {
		for _, reset := range []bool{false, true} {
			for _, withInstrument := range []bool{false, true} {
				t.Run(fmt.Sprintf("S%02X/reset=%t/instrument=%t", arg, reset, withInstrument), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, arg, 0}
					if withInstrument {
						rows = append(rows, 0x81, 3, 60, 2, 0, 0x81, 3, 60, 2, 0)
					} else {
						rows = append(rows, 0x81, 1, 60, 0, 0x81, 1, 60, 0)
					}
					b := mptS70Fixture(t, rows)
					instrument := binary.LittleEndian.Uint32(b[198:])
					binary.LittleEndian.PutUint16(b[instrument+20:], 128)
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITNNAReset)/8
					mask := byte(1 << (mptITNNAReset % 8))
					b[flag] &^= mask
					if reset {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					if rmsWindow(ref, 30, 35) < .001 {
						t.Fatal("reference is silent")
					}
					for ms := 20; ms < 230; ms += 10 {
						if ms%40 == 0 || ms%40 == 10 {
							continue
						}
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(got-want) > .005 {
							t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMS7NewNoteActionsInstrumentOnly(t *testing.T) {
	for _, control := range []byte{0x73, 0x74} {
		for _, reset := range []bool{false, true} {
			for _, instr := range []byte{1, 2} {
				t.Run(fmt.Sprintf("S%02X/reset=%t/instrument=%d", control, reset, instr), func(t *testing.T) {
					rows := []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, control, 0, 0x81, 2, instr, 0, 0x81, 1, 60, 0, 0x81, 1, 60, 0}
					b := mptS70Fixture(t, rows)
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITNNAReset)/8
					mask := byte(1 << (mptITNNAReset % 8))
					b[flag] &^= mask
					if reset {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					for _, ms := range []int{135, 150, 175, 190, 215} {
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(got-want) > .005 {
							t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMS7NewNoteActionsSameRowAndMemory(t *testing.T) {
	for _, control := range []byte{0x73, 0x74} {
		for _, reset := range []bool{false, true} {
			for _, memory := range []bool{false, true} {
				t.Run(fmt.Sprintf("S%02X/reset=%t/memory=%t", control, reset, memory), func(t *testing.T) {
					rows := []byte{0x81, 11, 60, 2, 19, control, 0, 0x81, 1, 60, 0}
					if memory {
						rows = append(rows, 0x81, 9, 60, 19, 0, 0)
					} else {
						rows = append(rows, 0x81, 1, 60, 0)
					}
					rows = append(rows, 0x81, 1, 60, 0)
					b := mptS70Fixture(t, rows)
					flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITNNAReset)/8
					mask := byte(1 << (mptITNNAReset % 8))
					b[flag] &^= mask
					if reset {
						b[flag] |= mask
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
					for _, ms := range []int{55, 75, 95, 115, 135, 155, 175, 195} {
						want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
						if math.Abs(got-want) > .005 {
							t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTMS7NewNoteActionsAfterCut(t *testing.T) {
	for _, control := range []byte{0x73, 0x74} {
		for _, reset := range []bool{false, true} {
			t.Run(fmt.Sprintf("S%02X/reset=%t", control, reset), func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, 254, 0, 0x81, 8, 19, control, 0, 0x81, 1, 60, 0, 0x81, 1, 60, 0}
				b := mptS70Fixture(t, rows)
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITNNAReset)/8
				mask := byte(1 << (mptITNNAReset % 8))
				b[flag] &^= mask
				if reset {
					b[flag] |= mask
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				assertMPTPatternDelayChunking(t, b, referenceRate/5)
				for _, ms := range []int{135, 150, 175, 190, 215} {
					want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
					if math.Abs(got-want) > .005 {
						t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
					}
				}
			})
		}
	}
}
