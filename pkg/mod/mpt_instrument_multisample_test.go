package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTMInstrumentWithoutNoteMultisampleFlag(t *testing.T) {
	for _, ended := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, multi := range []bool{false, true} {
				for _, instr := range []byte{1, 2} {
					t.Run(fmt.Sprintf("ended=%t/flag=%t/multi=%t/instrument=%d", ended, enabled, multi, instr), func(t *testing.T) {
						rows := []byte{0x81, 3, 60, 2, 0}
						if ended {
							rows = append(rows, 0, 0, 0, 0, 0, 0, 0)
						}
						rows = append(rows, 0x81, 2, instr, 0)
						b := mptInstrumentWithoutNoteFixture(t, rows, enabled, 0)
						field := bytesIndex(t, b, ".FSM")
						end := field + 6 + 7
						tail := binary.LittleEndian.Uint32(b[len(b)-4:])
						b = append(append(append([]byte(nil), b[:end]...), make([]byte, 6)...), b[end:]...)
						binary.LittleEndian.PutUint16(b[field+4:], 13)
						binary.LittleEndian.PutUint32(b[len(b)-4:], tail+6)
						if multi {
							b[field+6+10] |= 0x80
						}
						ref, actual := mptSharedEffectRenderForDuration(t, b, 750)
						for ms := 20; ms < 740; ms += 10 {
							if ms%60 == 0 {
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
