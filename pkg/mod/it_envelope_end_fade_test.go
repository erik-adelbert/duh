package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTVolumeEnvelopeEndFade(t *testing.T) {
	for _, fadeout := range []uint16{0, 64, 256} {
		for _, loop := range []bool{false, true} {
			t.Run(fmt.Sprintf("fadeout=%d/loop=%t", fadeout, loop), func(t *testing.T) {
				b := mptInstrumentWithoutNoteFixture(t, []byte{0x81, 3, 60, 2, 0}, false, 0)
				instrument := int(binary.LittleEndian.Uint32(b[198:]))
				binary.LittleEndian.PutUint16(b[instrument+20:], fadeout)
				flags := byte(1)
				if loop {
					flags |= 2
				}
				setTestEnvelope(b[instrument:instrument+554], 304, flags, []int8{64, 32}, []uint16{0, 3})
				if loop {
					b[instrument+306], b[instrument+307] = 0, 1
				}
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				b[sample+18] |= 0x10
				binary.LittleEndian.PutUint32(b[sample+52:], 0)
				binary.LittleEndian.PutUint32(b[sample+56:], 4096)
				ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
				for tick := 2; tick < 10; tick++ {
					ms := tick*20 + 15
					want, got := channelRMS(ref, ms, ms+3, 0), channelRMS(actual, ms, ms+3, 0)
					if math.Abs(got-want) > .003 {
						t.Errorf("tick %d: OpenMPT RMS %g, tracker %g", tick, want, got)
					}
				}
			})
		}
	}
}
