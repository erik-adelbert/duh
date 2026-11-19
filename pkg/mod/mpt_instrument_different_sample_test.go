package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMInstrumentWithoutNoteDifferentSample(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, porta := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				for _, nna := range []byte{0, 1} {
					for _, swapFlag := range []bool{false, true} {
						t.Run(fmt.Sprintf("flag=%t/porta=%t/compatible=%t/nna=%d/swapFlag=%t", enabled, porta, compatible, nna, swapFlag), func(t *testing.T) {
							row := []byte{0x81, 2, 1}
							if porta {
								row = []byte{0x81, 10, 1, 7, 4}
							}
							rows := append([]byte{0x81, 3, 60, 2, 0}, row...)
							rows = append(rows, 0)
							b := mptTwoPCMSamplesFixture(t, rows, enabled, false)
							flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITPortamentoInstrument)/8
							mask := byte(1 << (mptITPortamentoInstrument % 8))
							b[flag] &^= mask
							if swapFlag {
								b[flag] |= mask
							}
							instrument := binary.LittleEndian.Uint32(b[198:])
							b[instrument+17] = nna
							if compatible {
								b[44] |= 0x20
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
							for ms := 20; ms < 290; ms += 10 {
								if ms == 60 || ms == 70 {
									continue
								}
								want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
								if math.Abs(got-want) > .003 {
									t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
								}
							}
						})
					}
				}
			}
		}
	}
}
