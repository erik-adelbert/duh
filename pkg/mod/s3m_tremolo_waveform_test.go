package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MTremoloWaveform(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	for _, waveform := range []byte{0, 1, 2, 3} {
		t.Run(fmt.Sprint(waveform), func(t *testing.T) {
			data := referenceS3M(0, 0, sample)
			data[0x100+0x1c] = 32
			rows := []byte{0xa0, 0x40, 1, 19, 0x40 | waveform, 0,
				0x80, 18, 0x47, 0}
			rows = append(rows, make([]byte, 62)...)
			binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
			copy(data[0x152:], rows)
			baseline := append([]byte(nil), data...)
			baseline[0x152+7], baseline[0x152+8] = 0, 0
			ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
			refBase, actualBase := referencePCMWithChunks(t, baseline, "1", 240, 256)
			for _, ms := range []int{159, 179, 199, 219, 239} {
				want := rmsWindow(ref, ms, ms+1) / rmsWindow(refBase, ms, ms+1)
				got := rmsWindow(actual, ms, ms+1) / rmsWindow(actualBase, ms, ms+1)
				if math.Abs(got-want) > .06 {
					t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
