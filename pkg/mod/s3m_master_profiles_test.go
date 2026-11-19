package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTS3MMasterVolumeProfiles(t *testing.T) {
	tests := []struct {
		name   string
		master byte
		signed bool
		gus    bool
	}{
		{"signed legacy", 3, true, false},
		{"signed legacy mono", 0, true, false},
		{"gus ignores master", 32, false, true},
		{"gus ignores stereo master", 0xc0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := make([]byte, 4096)
			for i := range sample {
				sample[i] = 160
				if tt.signed {
					sample[i] = 32
				}
			}
			data := referenceS3M(0, 0, sample)
			data[0x33] = tt.master
			if tt.signed {
				binary.LittleEndian.PutUint16(data[0x2a:], 1)
			}
			if tt.gus {
				binary.LittleEndian.PutUint16(data[0x28:], 0x1301)
				binary.LittleEndian.PutUint16(data[0x128:], 2)
			}
			ref, actual := referencePCM(t, data)
			want, got := rmsWindow(ref, 15, 19), rmsWindow(actual, 15, 19)
			if math.Abs(got/want-1) > .03 {
				t.Errorf("OpenMPT RMS %.6f, tracker %.6f", want, got)
			}
		})
	}
}
