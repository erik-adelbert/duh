package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MDelayVolumeSlide(t *testing.T) {
	sample := make([]byte, 8192)
	for i := range sample {
		sample[i] = 192
	}
	data := referenceS3M(0, 0, sample)
	data[0x41] = 1
	rows := append([]byte{0xa0, 0x40, 1, 4, 0x02, 0x81, 19, 0xe1, 0}, make([]byte, 63)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	baseline := append([]byte(nil), data...)
	baseline[0x152+3], baseline[0x152+4] = 0, 0
	ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
	refBase, actualBase := referencePCMWithChunks(t, baseline, "1", 240, 256)
	for _, ms := range []int{15, 35, 115, 135, 155, 215} {
		want := rmsWindow(ref, ms, ms+4) / rmsWindow(refBase, ms, ms+4)
		got := rmsWindow(actual, ms, ms+4) / rmsWindow(actualBase, ms, ms+4)
		if math.Abs(got-want) > .01 {
			t.Errorf("%dms: OpenMPT ratio %.3f tracker %.3f", ms, want, got)
		}
	}
}
