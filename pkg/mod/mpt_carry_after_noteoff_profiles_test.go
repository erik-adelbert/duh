package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTCarryAfterNoteOffProfiles(t *testing.T) {
	for _, special := range []byte{253, 255} {
		for _, enabled := range []bool{false, true} {
			for _, reset := range []bool{false, true} {
				t.Run(fmt.Sprintf("special=%d/enabled=%t/reset=%t", special, enabled, reset), func(t *testing.T) {
					fixture := func(carry bool) []byte {
						b := mptCarryAfterNoteOffFixture(t, enabled, carry, special)
						if !reset {
							b[bytesIndex(t, b, ".FSM")+6+int(mptITEnvelopeReset)/8] &^= 1 << (mptITEnvelopeReset % 8)
						}
						return b
					}
					ref, actual := mptSharedEffectRenderForDuration(t, fixture(true), 240)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, fixture(false), 240)
					for tick := 7; tick < 11; tick++ {
						ms := tick*20 + 17
						want := channelRMS(ref, ms, ms+2, 0) / channelRMS(baseRef, ms, ms+2, 0)
						got := channelRMS(actual, ms, ms+2, 0) / channelRMS(baseActual, ms, ms+2, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .05 {
							t.Errorf("tick %d: OpenMPT ratio %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestMPTCarryAfterNoteOffVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01320000, true}, {0x01320039, false}, {0x01320040, true}, {0x01320400, true},
	} {
		b := mptCarryAfterNoteOffFixture(t, true, true, 255)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptCarryAfterNoteOff; got != tt.enabled {
			t.Errorf("version %#x: enabled=%t, want %t", tt.version, got, tt.enabled)
		}
	}
}
