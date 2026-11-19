package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMPatternDelayInstrumentWithoutNote(t *testing.T) {
	for _, delay := range [][]byte{{0x63}, {0xe2}, {0xe2, 0x63}} {
		for _, sameChannel := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				for _, instr := range []byte{1, 2} {
					t.Run(fmt.Sprintf("%X/sameChannel=%t/flag=%t/instrument=%d", delay, sameChannel, enabled, instr), func(t *testing.T) {
						rows := []byte{0x83, 3, 60, 2, 0}
						for channel, value := range delay {
							rows = append(rows, byte(0x81+channel), 8, 19, value)
						}
						rows = append(rows, 0x83, 2, instr)
						if sameChannel {
							rows = []byte{0x81, 3, 60, 2, 0, 0x81, 10, instr, 19, delay[0]}
							if len(delay) > 1 {
								rows = append(rows, 0x82, 8, 19, delay[1])
							}
						}
						rows = append(rows, 0)
						for _, speed := range []byte{1, 2, 3} {
							for _, firstTick := range []bool{false, true} {
								t.Run(fmt.Sprintf("speed=%d/firstTick=%t", speed, firstTick), func(t *testing.T) {
									b := mptInstrumentWithoutNoteFixture(t, rows, enabled, 0)
									b[50] = speed
									flag := bytesIndex(t, b, ".FSM") + 6 + int(mptITFirstTickHandling)/8
									mask := byte(1 << (mptITFirstTickHandling % 8))
									b[flag] &^= mask
									if firstTick {
										b[flag] |= mask
									}
									ref, actual := mptSharedEffectRenderForDuration(t, b, 450)
									for ms := 20; ms < 440; ms += 10 {
										if ms == int(speed)*20 {
											continue
										}
										want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
										if math.Abs(got-want) > .003 {
											t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
										}
									}
									assertMPTPatternDelayChunking(t, b, referenceRate/3)
								})
							}
						}
					})
				}
			}
		}
	}
}
