package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestMPTMEnvelopeCommandReuse(t *testing.T) {
	for _, arg := range []byte{0x77, 0x78, 0x7a} {
		pattern, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0, 0x81, 0x80, 0}, 2, true)
		if err != nil {
			t.Fatalf("S%02X: %v", arg, err)
		}
		for _, row := range pattern.Rows {
			if row[0].EffectCmd != 0x15 || row[0].EffectArg != arg {
				t.Fatalf("S%02X: unexpected command %+v", arg, row[0])
			}
		}
	}
}

func TestOpenMPTMEnvelopeControls(t *testing.T) {
	tests := []struct {
		name        string
		offset      int
		first, next byte
		values      []int8
		enabled     bool
	}{
		{"volume off on", 304, 0x77, 0x78, []int8{48, 24}, true},
		{"volume on off", 304, 0x78, 0x77, []int8{48, 24}, false},
		{"pan off on", 386, 0x79, 0x7a, []int8{-24, 24}, true},
		{"pan on off", 386, 0x7a, 0x79, []int8{-24, 24}, false},
	}
	for _, tt := range tests {
		for _, position := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/position=%t", tt.name, position), func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 2, 0, 0x81, 8, 19, tt.first, 0, 0x81, 8, 19, tt.next, 0}
				b := mptS70Fixture(t, rows)
				instrument := int(binary.LittleEndian.Uint32(b[198:]))
				envFlags := byte(0)
				if tt.enabled {
					envFlags = 1
				}
				setTestEnvelope(b[instrument:instrument+554], tt.offset, envFlags, tt.values, []uint16{0, 12})
				flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITEnvelopePositionHandling)/8
				if position {
					b[flag] |= 1 << (uint(mptITEnvelopePositionHandling) % 8)
				} else {
					b[flag] &^= 1 << (uint(mptITEnvelopePositionHandling) % 8)
				}
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				refBase, actualBase := channelRMS(ref, 18, 19, 0), channelRMS(actual, 18, 19, 0)
				if refBase < .001 || actualBase < .001 {
					t.Fatal("silent comparison baseline")
				}
				for tick := range 12 {
					ms := tick*20 + 18
					rl, rr := channelRMS(ref, ms, ms+1, 0), channelRMS(ref, ms, ms+1, 1)
					al, ar := channelRMS(actual, ms, ms+1, 0), channelRMS(actual, ms, ms+1, 1)
					want, got := rl/refBase, al/actualBase
					if tt.offset == 386 {
						want, got = rl/(rl+rr), al/(al+ar)
					}
					tolerance := .035
					if tt.offset == 304 {
						tolerance = .08
					}
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > tolerance {
						t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}
