package mod

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestMPTMTremorI11VersionUpgrade(t *testing.T) {
	flags := mptSavedPlayBehaviours((1 << mptKnownPlayBehaviourCount) - 1)
	for _, version := range []uint32{0x01291201, 0x01291202} {
		for _, compatible := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%08x/compatible=%t/old=%t", version, compatible, old), func(t *testing.T) {
					profile := flags
					if !compatible {
						profile &^= 1 << mptITTremor
					}
					b := mptSharedEffectFixture(t, []byte{0x81, 8, 9, 0x11, 0, 0x81, 0x80, 0}, profile)
					binary.LittleEndian.PutUint32(b[bytesIndex(t, b, "VWSL")+6:], version)
					if old {
						b[44] |= 0x10
					}
					loaded, err := loadMPTM(b)
					if err != nil {
						t.Fatal(err)
					}
					want := uint8(0x11)
					if version < 0x01291202 && compatible && !old {
						want = 0
					}
					for row := range 2 {
						cmd := loaded.Assets.Patterns[0].Rows[row][0]
						if cmd.EffectCmd != 0x22 || cmd.EffectArg != want {
							t.Fatalf("row %d: %+v, want I%02X", row, cmd, want)
						}
					}
				})
			}
		}
	}
}
