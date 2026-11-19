package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptStoppedFilterEnvelopeFixture(t *testing.T, enabled bool, cutoff, resonance byte) []byte {
	t.Helper()
	rows := []byte{0x81, 11, 60, 2, 19, 0x7b, 0}
	b := mptInstrumentWithoutNoteFixture(t, rows, true, 0)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	h := b[instrument : instrument+554]
	h[58], h[59] = 128|cutoff, 128|resonance
	setTestEnvelope(h, 468, 0x81, []int8{-32, 24}, []uint16{0, 12})
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01320400)
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	field := bytesIndex(t, b, ".FSM")
	length := int(binary.LittleEndian.Uint16(b[field+4:]))
	flags := make([]byte, 18)
	copy(flags, b[field+6:field+6+length])
	if enabled {
		flags[17] |= 1
	}
	out := append([]byte(nil), b[:field]...)
	out = append(out, mptExtensionField(".FSM", uint16(len(flags)), flags)...)
	out = append(out, b[field+6+length:bounds.serializedStart]...)
	serializedStart := len(out)
	out = append(out, b[bounds.serializedStart:bounds.serializedEnd]...)
	return binary.LittleEndian.AppendUint32(out, uint32(serializedStart))
}

func TestOpenMPTStoppedFilterEnvelopeAtStart(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, cutoff := range []byte{20, 80, 127} {
			for _, resonance := range []byte{0, 40} {
				t.Run(fmt.Sprintf("enabled=%t/cutoff=%d/resonance=%d", enabled, cutoff, resonance), func(t *testing.T) {
					b := mptStoppedFilterEnvelopeFixture(t, enabled, cutoff, resonance)
					ref, actual := mptSharedEffectRenderForDuration(t, b, 120)
					r, a := rmsWindow(ref, 30, 90), rmsWindow(actual, 30, 90)
					if math.Abs(r-a) > .005 {
						t.Fatalf("reference RMS %g, actual RMS %g", r, a)
					}
					assertMPTPCMCorrelation(t, ref, actual, 30, 90)
				})
			}
		}
	}
}
