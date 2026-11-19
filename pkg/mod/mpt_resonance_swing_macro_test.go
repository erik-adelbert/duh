package mod

import (
	"encoding/binary"
	"math"
	"math/rand"
	"testing"
)

func TestOpenMPTResonanceSwingMacro(t *testing.T) {
	random := rand.New(rand.NewSource(1)).Intn(256) - 128
	var base uint8
	var delta int16
	for value := 1; value <= 127; value++ {
		d := mptFilterSwingDelta(uint8(value), 64, random)
		if d != 0 && d%8 == 0 && d > -64 && d < 24 {
			base, delta = uint8(value), d
			break
		}
	}
	if base == 0 {
		t.Fatal("could not construct fixed-macro reference")
	}
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{false: "modern", true: "old"}[old], func(t *testing.T) {
			render := func(macro uint8) ([]float32, []float32) {
				flags := []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}
				if old {
					flags[0] |= 2
				}
				b := oplEnvelopeEndFixtureRows(t, 0x01280100, flags, 1, []byte{0x81, 8, 26, macro, 0})
				instrument := int(binary.LittleEndian.Uint32(b[198:]))
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				b[pattern+11] = 2
				b[instrument+304] = 0
				b[instrument+58], b[instrument+59] = 128|75, 128|base
				loaded, err := loadITData(b, true)
				if err != nil {
					t.Fatal(err)
				}
				profile, err := projectMPTMixProfile(4, 64, 64)
				if err != nil {
					t.Fatal(err)
				}
				loaded.Assets.ITCompatibility.mptMixProfile = &profile
				loaded.Assets.ITCompatibility.mptOldSwingBehaviour = old
				loaded.Assets.ITInstruments[1].mptResonanceSwing = 64
				w := NewTracker(loaded.Assets, referenceRate, 5760)
				w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 1, 125, -1
				actual := make([]float32, 5760*2)
				if n := w.Read(actual, 5760); n != 5760 {
					t.Fatalf("short render %d", n)
				}
				refData := append([]byte(nil), b...)
				refData[instrument+59] = 128 | mptApplyFilterSwing(base, delta)
				if old {
					refData[pattern+8+5+1+3] = uint8(int(macro) + int(delta)/8)
				}
				return referenceTransition(t, refData), actual
			}
			ref, actual := render(0x8c)
			refBase, actualBase := render(0x88)
			gotBase, wantBase := rmsWindow(actualBase, 65, 80), rmsWindow(refBase, 65, 80)
			if gotBase < .001 || wantBase < .001 {
				t.Fatal("silent macro baseline")
			}
			got, want := rmsWindow(actual, 65, 80)/gotBase, rmsWindow(ref, 65, 80)/wantBase
			if math.Abs(got-want) > .05 {
				t.Fatalf("macro delta %d relative RMS: tracker %g, OpenMPT %g", delta, got, want)
			}
		})
	}
}
