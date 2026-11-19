package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTPCMNoteFade(t *testing.T) {
	fixture := func(fadeout uint16) []byte {
		b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0x81, 1, 253, 0})
		instrument := int(binary.LittleEndian.Uint32(b[198:]))
		pattern := int(binary.LittleEndian.Uint32(b[210:]))
		b[pattern+11] = 2
		binary.LittleEndian.PutUint16(b[instrument+20:], fadeout)
		return b
	}
	render := func(b []byte) []float32 {
		loaded, err := loadITData(b, true)
		if err != nil {
			t.Fatal(err)
		}
		w := NewTracker(loaded.Assets, referenceRate, 5760)
		w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
		out := make([]float32, 5760*2)
		if n := w.Read(out, 5760); n != 5760 {
			t.Fatalf("short render %d", n)
		}
		return out
	}
	fade, base := fixture(256), fixture(0)
	actual, actualBase := render(fade), render(base)
	ref, refBase := referenceTransition(t, fade), referenceTransition(t, base)
	for _, start := range []int{45, 85, 105} {
		trackerBase, referenceBase := rmsWindow(actualBase, start, start+5), rmsWindow(refBase, start, start+5)
		if trackerBase < .001 || referenceBase < .001 {
			t.Fatalf("%dms: silent PCM baseline", start)
		}
		got := rmsWindow(actual, start, start+5) / trackerBase
		want := rmsWindow(ref, start, start+5) / referenceBase
		if math.Abs(got-want) > .05 {
			t.Fatalf("%dms: tracker fade ratio %g, OpenMPT %g", start, got, want)
		}
	}
}
