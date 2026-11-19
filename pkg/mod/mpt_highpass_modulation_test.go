package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTHighPassModulation(t *testing.T) {
	for _, kind := range []string{"envelope", "macro"} {
		t.Run(kind, func(t *testing.T) {
			b := mptFilterModeFixture(t, 1)
			if kind == "envelope" {
				instrument := int(binary.LittleEndian.Uint32(b[198:]))
				setTestEnvelope(b[instrument:instrument+554], 468, 0x81, []int8{-32, 32, -16}, []uint16{0, 3, 5})
			} else {
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				rows := []byte{0x81, 3, 60, 2, 0, 0x81, 8, 26, 40, 0, 0x81, 8, 26, 100, 0}
				rows = append(rows, make([]byte, 61)...)
				oldLength := int(binary.LittleEndian.Uint16(b[pattern:]))
				oldTail := binary.LittleEndian.Uint32(b[len(b)-4:])
				data := append([]byte(nil), b[:pattern+8]...)
				binary.LittleEndian.PutUint16(data[pattern:], uint16(len(rows)))
				data = append(data, rows...)
				data = append(data, b[pattern+8+oldLength:len(b)-4]...)
				b = binary.LittleEndian.AppendUint32(data, oldTail+uint32(len(rows)-oldLength))
			}
			ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
			for _, window := range [][2]int{{15, 30}, {55, 70}, {95, 110}, {130, 145}} {
				var cross, r2, a2 float64
				for i := window[0] * 96; i < window[1]*96; i++ {
					r, a := float64(ref[i]), float64(actual[i])
					cross, r2, a2 = cross+r*a, r2+r*r, a2+a*a
				}
				correlation, gain := cross/math.Sqrt(r2*a2), math.Sqrt(a2/r2)
				if r2 < .00001 || a2 < .00001 || math.IsNaN(correlation) || correlation < .96 || math.Abs(gain-1) > .08 {
					t.Fatalf("%v ms: correlation %g gain %g energy %g/%g", window, correlation, gain, r2, a2)
				}
			}
		})
	}
}
