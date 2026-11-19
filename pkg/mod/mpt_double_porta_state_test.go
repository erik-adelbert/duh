package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestMPTDoublePortamentoVersion(t *testing.T) {
	for _, tt := range []struct {
		version uint32
		enabled bool
	}{
		{0x01320000, true}, {0x01320026, false}, {0x01320027, true}, {0x01320400, true},
	} {
		b := mptCarryAfterNoteOffFixture(t, false, false, 255)
		b[bytesIndex(t, b, ".FSM")+6+mptITDoublePortamentoSlidesBit/8] |= 1 << (mptITDoublePortamentoSlidesBit % 8)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], tt.version)
		loaded, err := loadMPTM(b)
		if err != nil {
			t.Fatal(err)
		}
		if got := loaded.Assets.ITCompatibility.mptDoublePortamentoSlides; got != tt.enabled {
			t.Errorf("version %#x: enabled=%t, want %t", tt.version, got, tt.enabled)
		}
	}
}

func TestOpenMPTDoublePortamentoFlagChangesReference(t *testing.T) {
	b := mptCarryAfterNoteOffFixture(t, false, false, 255)
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	length := int(binary.LittleEndian.Uint16(b[pattern:]))
	binary.LittleEndian.PutUint16(b[pattern+2:], 32)
	clear(b[pattern+8 : pattern+8+length])
	copy(b[pattern+8:], []byte{0x81, 3, 60, 2, 0, 0x81, 13, 64, 197, 6, 8, 0})
	a := referenceMPTMForDuration(t, b, 160)
	b[bytesIndex(t, b, ".FSM")+6+mptITDoublePortamentoSlidesBit/8] |= 1 << (mptITDoublePortamentoSlidesBit % 8)
	ref, actual := mptSharedEffectRenderForDuration(t, b, 160)
	assertMPTPCMCorrelation(t, ref, actual, 80, 110)
	var delta float64
	for i := 80 * 96; i < 110*96; i++ {
		d := float64(ref[i] - a[i])
		delta += d * d
	}
	if math.IsNaN(delta) || delta < .01 {
		t.Fatalf("reference does not distinguish bit 127: squared difference %g", delta)
	}
}
