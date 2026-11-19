package mod

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestMPTMSharedEffectsProfileValidation(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, effect := range []byte{9, 11, 12, 14, 16, 18, 21, 23} {
		b := mptSharedEffectFixture(t, []byte{0x81, 8, effect, 1, 0}, flags)
		binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
		if _, err := loadMPTM(b); err == nil || !strings.Contains(err.Error(), "pre-1.26") {
			t.Errorf("effect %d: got %v, want pre-1.26 profile error", effect, err)
		}
	}
	b := mptSharedEffectFixture(t, []byte{0x81, 8, 8, 1, 0}, flags)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01250000)
	if _, err := loadMPTM(b); err != nil {
		t.Fatalf("previously supported effect rejected: %v", err)
	}
	for _, volume := range []byte{193, 202} {
		_, err := unpackITPatternMode([]byte{0x81, 12, volume, 12, 1, 0}, 1, true)
		if err == nil || !strings.Contains(err.Error(), "volume tone portamento with Lxx") {
			t.Errorf("volume %d with Lxx: got %v", volume, err)
		}
	}
}
