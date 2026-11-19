package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTMTremorTremoloSpeedOne(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"tremor", []byte{0x81, 11, 60, 1, 9, 0x23, 0, 0x81, 8, 9, 0, 0, 0x81, 8, 9, 0, 0,
			0x81, 8, 9, 0, 0, 0x81, 8, 9, 0, 0, 0x81, 8, 9, 0, 0}},
		{"tremolo", []byte{0x81, 15, 60, 1, 32, 18, 0x4f, 0, 0x81, 8, 18, 0, 0, 0x81, 8, 18, 0, 0,
			0x81, 8, 18, 0, 0, 0x81, 8, 18, 0, 0, 0x81, 8, 18, 0, 0}},
	}
	all := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, tt := range tests {
		for _, compatible := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/compatible=%t/old=%t", tt.name, compatible, old), func(t *testing.T) {
					flags := all
					if !compatible {
						flags &^= 1<<mptITTremor | 1<<mptITVibratoTremoloPanbrello
					}
					fixture := func(rows []byte) []byte {
						b := mptSharedEffectFixture(t, rows, flags)
						b[50] = 1
						if old {
							b[44] |= 0x10
						}
						return b
					}
					ref, actual := mptSharedEffectRender(t, fixture(tt.rows))
					baseRef, baseActual := mptSharedEffectRender(t, fixture([]byte{0x81, 7, 60, 1, 32, 0}))
					for tick := range 6 {
						assertMPTMVolumeRatio(t, ref, actual, baseRef, baseActual, tick, .05)
					}
				})
			}
		}
	}
}
