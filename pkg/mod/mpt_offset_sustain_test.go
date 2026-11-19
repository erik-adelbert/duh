package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTOffsetWithInstrumentAfterSustain(t *testing.T) {
	for _, bit122 := range []bool{false, true} {
		for _, bit124 := range []bool{false, true} {
			for _, instrument := range []byte{0, 1} {
				t.Run(fmt.Sprintf("bit122=%t/bit124=%t/instrument=%d", bit122, bit124, instrument), func(t *testing.T) {
					tail := []byte{0x81, 10, 2, 15, 1, 0}
					b := mptSustainPortaFixtureWithTail(t, bit122, false, true, instrument, true, tail)
					if bit124 {
						b[bytesIndex(t, b, ".FSM")+6+mptITOffsetWithInstrumentBit/8] |= 1 << (mptITOffsetWithInstrumentBit % 8)
					}
					ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
					assertMPTPCMCorrelation(t, ref, actual, 200, 240)
				})
			}
		}
	}
}
