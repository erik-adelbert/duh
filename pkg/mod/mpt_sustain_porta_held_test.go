package mod

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestOpenMPTSustainPortamentoHeldLoopModes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			for _, pingPong := range []bool{false, true} {
				t.Run(fmt.Sprintf("enabled=%t/compatible=%t/pingpong=%t", enabled, compatible, pingPong), func(t *testing.T) {
					b := mptSustainPortaFixtureWithRelease(t, enabled, compatible, true, 1, false)
					for _, off := range []int{202, 206} {
						sample := int(binary.LittleEndian.Uint32(b[off:]))
						b[sample+18] |= 0x50
						if pingPong {
							b[sample+18] |= 0x80
						}
						binary.LittleEndian.PutUint32(b[sample+52:], 240)
						binary.LittleEndian.PutUint32(b[sample+56:], 320)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					assertMPTPCMCorrelation(t, ref, actual, 140, 180)
				})
			}
		}
	}
}
