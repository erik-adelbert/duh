package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTMGlobalSlideChannelMemory(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprintf("shared=%t", shared), func(t *testing.T) {
			profile := flags
			if shared {
				profile &^= 1 << mptPerChannelGlobalVolSlide
			}
			// Channel 1 stores W03; channel 2 recalls W00 on the next row.
			rows := []byte{0x81, 11, 60, 1, 23, 3, 0, 0x82, 8, 23, 0, 0}
			b := mptSharedEffectFixture(t, rows, profile)
			b[bytesIndex(t, b, "...C")+6] = 2
			base := mptSharedEffectFixture(t, []byte{0x81, 3, 60, 1, 0}, profile)
			base[bytesIndex(t, base, "...C")+6] = 2
			ref, actual := mptSharedEffectRender(t, b)
			baseRef, baseActual := mptSharedEffectRender(t, base)
			for tick := range 6 {
				assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .015)
			}
		})
	}
}
