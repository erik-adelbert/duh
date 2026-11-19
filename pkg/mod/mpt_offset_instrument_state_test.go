package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMPTOffsetWithInstrumentVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01310000, false}, {0x01320000, true}, {0x01320014, false},
		{0x01320015, true}, {0x01320400, true},
	} {
		b := mptOffsetInstrumentFixture(t, nil, true, true)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptOffsetWithInstrument; got != tt.enabled {
			t.Errorf("version %#x: enabled=%t, want %t", tt.version, got, tt.enabled)
		}
	}
}

func TestOpenMPTOffsetWithInstrumentFlagChangesReference(t *testing.T) {
	rows := []byte{0x81, 3, 60, 2, 0, 0x81, 10, 2, 15, 1, 0}
	var reference [2][]float32
	for i := range reference {
		b := mptOffsetInstrumentFixture(t, rows, i == 1, true)
		ref, actual := mptSharedEffectRenderForDuration(t, b, 220)
		assertMPTPCMCorrelation(t, ref, actual, 80, 110)
		reference[i] = ref
	}
	var difference float64
	for i := 80 * 96; i < 110*96; i++ {
		d := float64(reference[0][i] - reference[1][i])
		difference += d * d
	}
	if math.IsNaN(difference) || difference < 0.1 {
		t.Fatalf("reference does not distinguish bit 124: squared difference %g", difference)
	}
}
