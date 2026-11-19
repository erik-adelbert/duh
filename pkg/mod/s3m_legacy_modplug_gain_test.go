package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MLegacyModPlugMonoFlag(t *testing.T) {
	sample := make([]byte, 4096)
	for i := range sample {
		sample[i] = 160
	}
	data := referenceS3M(0, 0, sample)
	data[0x33], data[0x35] = 32, 0xfc
	binary.LittleEndian.PutUint16(data[0x20:], 16)
	for i := 1; i < 16; i++ {
		data[0x60+i] = 0xff
	}
	binary.LittleEndian.PutUint16(data[0x70:], 0x10)
	binary.LittleEndian.PutUint16(data[0x72:], 0x15)
	data[0x74] = 0x28
	ref, actual := referencePCM(t, data)
	want, got := rmsWindow(ref, 15, 19), rmsWindow(actual, 15, 19)
	if math.Abs(got/want-1) > .03 {
		t.Errorf("OpenMPT RMS %.6f, tracker %.6f", want, got)
	}
}
