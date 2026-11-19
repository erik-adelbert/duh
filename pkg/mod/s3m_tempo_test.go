package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTS3MTempoSlide(t *testing.T) {
	for _, arg := range []byte{0x02, 0x13, 0x1f} {
		t.Run(fmt.Sprintf("T%02X", arg), func(t *testing.T) {
			testOpenMPTS3MTempoSlide(t, arg)
		})
	}
}

func testOpenMPTS3MTempoSlide(t *testing.T, arg byte) {
	t.Helper()
	sample := make([]byte, 700)
	for i := range sample {
		sample[i] = 224
	}
	data := referenceS3M(0, 0, sample)
	binary.LittleEndian.PutUint32(data[0x100+0x20:], 48000)
	rows := []byte{0xa0, 0x40, 1, 20, arg, 0,
		0xa0, 0x40, 1, 20, 0, 0,
		0x20, 0x40, 1, 0}
	rows = append(rows, make([]byte, 61)...)
	binary.LittleEndian.PutUint16(data[0x150:], uint16(len(rows)))
	copy(data[0x152:], rows)
	ref, actual := referencePCMWithChunks(t, data, "1", 360, 256)
	onsets := func(pcm []float32) []int {
		var found []int
		active := false
		for i := 0; i < len(pcm)/2; i++ {
			next := math.Abs(float64(pcm[2*i])) > .001
			if next && !active {
				found = append(found, i)
			}
			active = next
		}
		return found
	}
	want, got := onsets(ref), onsets(actual)
	if len(want) < 3 || len(got) < 3 {
		t.Fatalf("insufficient note onsets: OpenMPT %v tracker %v", want, got)
	}
	for i := 1; i < 3; i++ {
		if delta := got[i] - want[i]; delta < -20 || delta > 20 {
			t.Errorf("onset %d: OpenMPT %d tracker %d frames", i, want[i], got[i])
		}
	}
}
