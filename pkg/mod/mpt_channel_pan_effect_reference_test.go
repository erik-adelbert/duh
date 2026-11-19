package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTChannelPanEffectReplacesRestore(t *testing.T) {
	flags := make([]byte, 13)
	copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
	flags[12] = 0x40
	b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{
		0x81, 8, 24, 192, 0,
		0x81, 3, 60, 2, 0,
	})
	first := int(binary.LittleEndian.Uint32(b[194:]))
	second := int(binary.LittleEndian.Uint32(b[198:]))
	b[first+304], b[first+25], b[second+25] = 0, 0, 0x80
	for note := range 120 {
		b[first+65+note*2] = 1
	}
	loaded, err := loadITData(b, true)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Assets.ITCompatibility.itRestoreChannelPan = true
	w := NewTracker(loaded.Assets, referenceRate, 128)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
	actual := make([]float32, 5760*2)
	for frame := 0; frame < 5760; frame += 128 {
		if n := w.Read(actual[frame*2:(frame+128)*2], 128); n != 128 {
			t.Fatalf("short render %d", n)
		}
	}
	ref := referenceTransition(t, b)
	for _, ms := range []int{92, 112} {
		balance := func(pcm []float32) float64 {
			l, r := channelRMS(pcm, ms, ms+7, 0), channelRMS(pcm, ms, ms+7, 1)
			if l+r < .001 {
				t.Fatal("silent comparison")
			}
			return (r - l) / (r + l)
		}
		got, want := balance(actual), balance(ref)
		if math.Abs(got-want) > .04 {
			t.Errorf("%dms: tracker balance %.4f, OpenMPT %.4f", ms, got, want)
		}
	}
}
