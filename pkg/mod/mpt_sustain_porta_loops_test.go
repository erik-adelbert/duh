package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTSustainPortamentoLoopModes(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			for _, swap := range []bool{false, true} {
				for _, sustainPingPong := range []bool{false, true} {
					for _, normal := range []byte{0, 0x10, 0x50} {
						name := fmt.Sprintf("enabled=%t/compatible=%t/swap=%t/pingpong=%t/normal=%02x", enabled, compatible, swap, sustainPingPong, normal)
						t.Run(name, func(t *testing.T) {
							b := mptSustainPortaFixture(t, enabled, compatible, swap, 1)
							offsets := []int{202}
							if swap {
								offsets = append(offsets, 206)
							}
							for _, off := range offsets {
								sample := int(binary.LittleEndian.Uint32(b[off:]))
								b[sample+18] |= normal
								if sustainPingPong {
									b[sample+18] |= 0x80
								}
								if normal != 0 {
									binary.LittleEndian.PutUint32(b[sample+52:], 240)
									binary.LittleEndian.PutUint32(b[sample+56:], 320)
								}
							}
							ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
							assertMPTPCMCorrelation(t, ref, actual, 140, 180)
						})
					}
				}
			}
		}
	}
}

func TestOpenMPTSustainPortamentoFlagChangesReference(t *testing.T) {
	var reference [2][]float32
	for i := range reference {
		b := mptSustainPortaFixture(t, i == 1, false, true, 1)
		ref, actual := mptSharedEffectRenderForDuration(t, b, 240)
		assertMPTPCMCorrelation(t, ref, actual, 140, 180)
		reference[i] = ref
	}
	var difference float64
	for i := 140 * 96; i < 180*96; i++ {
		d := float64(reference[0][i] - reference[1][i])
		difference += d * d
	}
	if math.IsNaN(difference) || difference < 0.1 {
		t.Fatalf("reference does not distinguish bit 122: squared difference %g", difference)
	}
}
