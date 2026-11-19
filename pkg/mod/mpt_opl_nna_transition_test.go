package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTOPLNNAIntoPCM(t *testing.T) {
	for _, action := range []byte{0, 1, 2, 3} {
		t.Run(fmt.Sprint(action), func(t *testing.T) {
			fixture := func(nna byte) ([]byte, *S3MLoadResult) {
				flags := make([]byte, 14)
				copy(flags, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7})
				flags[12], flags[13] = 8, 2
				b := oplEnvelopeEndFixture(t, 0x01300600, flags, []byte{0x81, 7, 72, 2, 0, 0})
				inst := int(binary.LittleEndian.Uint32(b[194:]))
				b[inst+17], b[inst+304] = nna, 0
				binary.LittleEndian.PutUint16(b[inst+20:], 0)
				sample := int(binary.LittleEndian.Uint32(b[206:]))
				patch := int(binary.LittleEndian.Uint32(b[sample+72:]))
				copy(b[patch:patch+12], []byte{0x21, 0x21, 0x3f, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0})
				loaded, err := loadITData(b, true)
				if err != nil {
					t.Fatal(err)
				}
				loaded.Assets.ITCompatibility.oplFlexibleNoteOff = true
				return b, loaded
			}
			module, loaded := fixture(action)
			base, baseLoaded := fixture(1)
			render := func(loaded *S3MLoadResult) []float32 {
				w := NewTracker(loaded.Assets, referenceRate, 128)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
				out := make([]float32, 11520)
				for frame := 0; frame < 5760; frame += 128 {
					if w.Read(out[2*frame:2*(frame+128)], 128) != 128 {
						t.Fatal("short render")
					}
				}
				return out
			}
			actual, actualBase := render(loaded), render(baseLoaded)
			ref, refBase := referenceTransition(t, module), referenceTransition(t, base)
			wantBase, gotBase := transitionTone(refBase, 261.625), transitionTone(actualBase, 261.625)
			if wantBase < .001 || gotBase < .001 {
				t.Fatal("silent OPL continue baseline")
			}
			want, got := transitionTone(ref, 261.625)/wantBase, transitionTone(actual, 261.625)/gotBase
			if math.Abs(got-want) > .12 {
				t.Fatalf("old tone ratio: tracker %g, OpenMPT %g", got, want)
			}
		})
	}
}
