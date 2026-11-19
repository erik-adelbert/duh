package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func mptTwoPCMSamplesFixture(t *testing.T, rows []byte, enabled, sampleMode bool) []byte {
	t.Helper()
	b := mptInstrumentWithoutNoteFixture(t, rows, enabled, 0)
	sample1 := binary.LittleEndian.Uint32(b[202:])
	sample2 := binary.LittleEndian.Uint32(b[206:])
	pattern := binary.LittleEndian.Uint32(b[210:])
	bounds, err := locateMPTExtensions(b)
	if err != nil {
		t.Fatal(err)
	}
	songProperties := bounds.propertiesStart
	if sampleMode {
		songProperties = bytesIndex(t, b, "STPM")
	}
	copy(b[sample2:sample2+80], b[sample1:sample1+80])
	b[sample2+19] = 32
	sampleData := binary.LittleEndian.Uint32(b[sample1+72:])
	pcm := append([]byte(nil), b[sampleData:sampleData+8192]...)
	binary.LittleEndian.PutUint32(b[sample2+72:], uint32(bounds.propertiesStart))
	tail := binary.LittleEndian.Uint32(b[len(b)-4:])
	removed := songProperties - bounds.propertiesStart
	b = append(append(append([]byte(nil), b[:bounds.propertiesStart]...), pcm...), b[songProperties:]...)
	binary.LittleEndian.PutUint32(b[len(b)-4:], tail+uint32(len(pcm)-removed))
	if sampleMode {
		b[44] &^= 4
		binary.LittleEndian.PutUint16(b[34:], 0)
		binary.LittleEndian.PutUint32(b[194:], sample1)
		binary.LittleEndian.PutUint32(b[198:], sample2)
		binary.LittleEndian.PutUint32(b[202:], pattern)
	} else {
		instrument := binary.LittleEndian.Uint32(b[194:])
		for i := range 120 {
			b[instrument+65+uint32(i)*2] = 2
		}
	}
	return b
}

func TestOpenMPTMSampleWithoutNote(t *testing.T) {
	for _, delay := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			for _, target := range []byte{1, 2} {
				for _, prior := range []byte{0, 254} {
					t.Run(fmt.Sprintf("delay=%t/flag=%t/sample=%d/prior=%d", delay, enabled, target, prior), func(t *testing.T) {
						row := []byte{0x81, 2, target}
						if delay {
							row = append(row, 0x82, 8, 19, 0xe2)
						}
						rows := []byte{0x81, 3, 60, 1, 0}
						if prior != 0 {
							rows = append(rows, 0x81, 1, prior, 0)
						}
						rows = append(rows, row...)
						rows = append(rows, 0)
						b := mptTwoPCMSamplesFixture(t, rows, enabled, true)
						ref, actual := mptSharedEffectRenderForDuration(t, b, 300)
						for ms := 20; ms < 290; ms += 10 {
							if ms == 60 || prior != 0 && (ms == 120 || ms == 130) {
								continue
							}

							want, got := rmsWindow(ref, ms, ms+5), rmsWindow(actual, ms, ms+5)
							if math.Abs(got-want) > .003 {
								t.Errorf("%dms: OpenMPT RMS %g tracker %g", ms, want, got)
							}
						}
					})
				}
			}
		}
	}
}
