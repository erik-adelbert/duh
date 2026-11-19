package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMPTStoppedFilterEnvelopeFlagVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01320000, true}, {0x01320303, false}, {0x01320304, true}, {0x01320400, true},
	} {
		b := mptStoppedFilterEnvelopeFixture(t, true, 80, 40)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptStoppedFilterEnvAtStart; got != tt.enabled {
			t.Errorf("version %#x: enabled=%t, want %t", tt.version, got, tt.enabled)
		}
	}
}

func TestOpenMPTStoppedFilterEnvelopeFlagChangesReference(t *testing.T) {
	a := referenceMPTMForDuration(t, mptStoppedFilterEnvelopeFixture(t, false, 80, 40), 120)
	b := referenceMPTMForDuration(t, mptStoppedFilterEnvelopeFixture(t, true, 80, 40), 120)
	var delta float64
	for i := 30 * 96; i < 90*96; i++ {
		d := float64(a[i] - b[i])
		delta += d * d
	}
	if math.IsNaN(delta) || delta < .01 {
		t.Fatalf("reference does not distinguish bit 136: squared difference %g", delta)
	}
}
