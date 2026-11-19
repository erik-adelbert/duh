package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITLegacyNotePitch(t *testing.T) {
	for _, tt := range []struct {
		name   string
		note   byte
		speed  uint32
		linear bool
	}{
		{"nonlinear C5", 60, 22050, false},
		{"nonlinear G4", 55, 27776, false},
		{"nonlinear high note", 79, 10404, false},
		{"linear G4", 55, 27776, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows := []byte{0x81, 3, tt.note, 1, 0}
			b := testITPitchFixture(rows, 6, tt.linear, true)
			binary.LittleEndian.PutUint32(b[256+60:], tt.speed)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			want := referenceTickFrequency(t, ref, 4)
			got := referenceTickFrequency(t, actual, 4)
			if math.Abs(got/want-1) > 0.0003 {
				t.Fatalf("frequency OpenMPT %.3f Hz, tracker %.3f Hz", want, got)
			}
		})
	}
}
