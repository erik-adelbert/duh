package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestMPTMissingFlagsLegacyPitch(t *testing.T) {
	b := mptSharedEffectFixture(t, []byte{0x81, 11, 60, 1, 6, 8, 0}, 0)
	binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], 0x01230400)
	missing := mptWithoutPlaybackFlags(t, b)
	if _, err := loadMPTM(missing); err == nil || !strings.Contains(err.Error(), "pitch effect with legacy linear periods") {
		t.Fatalf("pre-1.24 pitch effect: %v", err)
	}
	binary.LittleEndian.PutUint32(missing[bytesIndex(t, missing, "VWSL")+6:], 0x01240000)
	ref, actual := mptSharedEffectRenderForDuration(t, missing, 120)
	want, got := referenceTickFrequency(t, ref, 4), referenceTickFrequency(t, actual, 4)
	if math.Abs(got-want)/want > .01 {
		t.Fatalf("1.24 linear pitch: tracker %g, OpenMPT %g", got, want)
	}
}
