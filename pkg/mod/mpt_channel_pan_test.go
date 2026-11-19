package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTInstrumentChannelPanRestore(t *testing.T) {
	for _, tt := range []struct{ restore, samplePan bool }{
		{false, false}, {true, false}, {true, true},
	} {
		restore, samplePan := tt.restore, tt.samplePan
		name := "legacy"
		if restore {
			name = "restore"
		}
		if samplePan {
			name += "/sample-priority"
		}
		t.Run(name, func(t *testing.T) {
			flags := make([]byte, 13)
			copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
			if restore {
				flags[12] = 0x40
			}
			b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{0x81, 3, 60, 2, 0})
			first := int(binary.LittleEndian.Uint32(b[194:]))
			second := int(binary.LittleEndian.Uint32(b[198:]))
			b[first+304] = 0
			b[first+25], b[second+25] = 0, 0x80
			for note := range 120 {
				b[first+65+note*2] = 1
			}
			if samplePan {
				sample := int(binary.LittleEndian.Uint32(b[202:]))
				b[sample+47] = 0x80 | 48
			}
			loaded, err := loadITData(b, true)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITCompatibility.itRestoreChannelPan = restore
			w := NewTracker(loaded.Assets, referenceRate, 5760)
			w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
			actual := make([]float32, 5760*2)
			if n := w.Read(actual, 5760); n != 5760 {
				t.Fatalf("short render %d", n)
			}
			ref := referenceTransition(t, b)
			for _, ms := range []int{45, 85} {
				balance := func(pcm []float32) float64 {
					l, r := channelRMS(pcm, ms, ms+10, 0), channelRMS(pcm, ms, ms+10, 1)
					if l+r < .001 {
						t.Fatalf("silent at %dms", ms)
					}
					return (r - l) / (r + l)
				}
				got, want := balance(actual), balance(ref)
				if math.Abs(got-want) > .04 {
					t.Errorf("%dms: tracker balance %.4f, OpenMPT %.4f", ms, got, want)
				}
			}
		})
	}
}
