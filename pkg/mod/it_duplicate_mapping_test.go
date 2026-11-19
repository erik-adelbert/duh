package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITDuplicateNoteMappingOpenMPT(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			fixture := func(check byte) []byte {
				b := testITDuplicateSampleFixture(check, 1, 1, legacy, 1)
				h := b[len(b)-1108:]
				h[64+61*2] = 60
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 3, 61, 1, 0}
				packed := append(rows, make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				for i := range 4096 {
					binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
				}
				return b
			}
			ref, actual := referencePCMWithChunks(t, fixture(1), "1", 80, 256)
			refBase, actualBase := referencePCMWithChunks(t, fixture(0), "1", 80, 256)
			want := channelRMS(ref, 78, 79, 0) / channelRMS(refBase, 78, 79, 0)
			got := channelRMS(actual, 78, 79, 0) / channelRMS(actualBase, 78, 79, 0)
			if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .06 {
				t.Errorf("OpenMPT ratio %.3f tracker %.3f", want, got)
			}
		})
	}
}
