package mod

import (
	"encoding/binary"
	"testing"
)

func TestOpenMPTS3MHighOffset(t *testing.T) {
	sample := make([]byte, 70000)
	for i := range sample {
		sample[i] = 128
		if i >= 65536 {
			sample[i] = 224
		}
	}
	data := referenceS3M(0, 0, sample)
	rows := []byte{0x80, 19, 0xa1, 0, 0xa0, 0x40, 1, 15, 0, 0}
	rows = append(rows, make([]byte, 62)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	ref, actual := referencePCMWithChunks(t, data, "1", 240, 256)
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		if level := rmsWindow(pcm, 135, 145); level < .005 {
			t.Errorf("%s: sample high offset missing, RMS=%g", name, level)
		}
	}
}
