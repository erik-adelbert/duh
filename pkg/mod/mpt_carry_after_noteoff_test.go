package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptCarryAfterNoteOffFixture(t *testing.T, enabled, carry bool, special byte) []byte {
	t.Helper()
	rows := []byte{0x81, 3, 60, 2, 0, 0x81, 1, special, 0, 0x81, 3, 60, 2, 0}
	b := mptInstrumentWithoutNoteFixture(t, rows, true, 0)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	flags := byte(1)
	if carry {
		flags |= 8
	}
	setTestEnvelope(b[instrument:instrument+554], 304, flags, []int8{64, 16}, []uint16{0, 12})
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01320400)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	flagsData := make([]byte, 17)
	copy(flagsData, b[field+6:field+6+length])
	if enabled {
		flagsData[mptITCarryAfterNoteOffBit/8] |= 1 << (mptITCarryAfterNoteOffBit % 8)
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flagsData)), flagsData)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	serializedStart := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(serializedStart))
}

func TestOpenMPTCarryAfterNoteOff(t *testing.T) {
	for _, special := range []byte{253, 255} {
		for _, enabled := range []bool{false, true} {
			for _, carry := range []bool{false, true} {
				t.Run(fmt.Sprintf("special=%d/enabled=%t/carry=%t", special, enabled, carry), func(t *testing.T) {
					b := mptCarryAfterNoteOffFixture(t, enabled, carry, special)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
					baseRef, baseActual := mptSharedEffectRenderForDuration(t, mptCarryAfterNoteOffFixture(t, enabled, false, special), 240)
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
