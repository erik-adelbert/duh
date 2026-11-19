package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestOpenMPTOPLFilterEnvelope(t *testing.T) {
	for _, tt := range []struct {
		cutoff byte
		swing  byte
		macro  bool
	}{
		{30, 0, false}, {64, 0, false}, {100, 0, false},
		{127, 0, false}, {94, 26, false}, {48, 0, true},
	} {
		t.Run(fmt.Sprintf("cutoff=%d/swing=%d/macro=%t", tt.cutoff, tt.swing, tt.macro), func(t *testing.T) {
			fixture := func(envelope bool, swing byte, cutoff byte) ([]byte, []float32) {
				b := oplEnvelopeEndFixture(t, 0x01280100, []byte{0x81, 0xff, 0xff, 0xff, 0xff, 0xff, 7}, []byte{0})
				b[30], b[44], b[50] = 8, b[44]|8, 2
				instrument := int(binary.LittleEndian.Uint32(b[194:]))
				b[instrument+304] = 0
				b[instrument+58] = 0x80 | cutoff
				if envelope {
					setTestEnvelope(b[instrument:instrument+554], 468, 0x81, []int8{-32, 0, 24}, []uint16{0, 2, 4})
				}
				sample := int(binary.LittleEndian.Uint32(b[206:]))
				patchOffset := int(binary.LittleEndian.Uint32(b[sample+72:]))
				patch := oplPatch{0x21, 0x21, 0, 0, 0xf0, 0xf0, 0x0f, 0x0f, 0, 0, 0, 0}
				copy(b[patchOffset:patchOffset+12], patch[:])
				pattern := int(binary.LittleEndian.Uint32(b[210:]))
				rows := []byte{0x81, 3, 60, 1, 0}
				if tt.macro {
					rows = []byte{0x81, 3, 60, 1, 0, 0x81, 8, 26, cutoff, 0}
					b[instrument+58] = 0xff
				}
				length := int(binary.LittleEndian.Uint16(b[pattern:]))
				binary.LittleEndian.PutUint16(b[pattern+2:], 32)
				clear(b[pattern+8 : pattern+8+length])
				copy(b[pattern+8:pattern+8+length], rows)
				return renderMPTFilterInstrumentFixture(t, b, []byte{swing, 0})
			}
			_, actual := fixture(true, tt.swing, tt.cutoff)
			effectiveCutoff := tt.cutoff
			if tt.swing != 0 {
				effectiveCutoff = mptRandomCutoff(tt.cutoff, tt.swing, rand.New(rand.NewSource(1)).Intn(256)-128)
			}
			refData, _ := fixture(true, 0, effectiveCutoff)
			base, baseline := fixture(false, 0, 127)
			ref, refBase := referenceTransition(t, refData), referenceTransition(t, base)
			for tick := range 6 {
				start := tick*20 + 4
				wantBase, gotBase := rmsWindow(refBase, start, start+12), rmsWindow(baseline, start, start+12)
				if wantBase < .00001 || gotBase < .00001 {
					t.Fatalf("tick %d: silent baseline", tick)
				}
				want := rmsWindow(ref, start, start+12) / wantBase
				got := rmsWindow(actual, start, start+12) / gotBase
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .12 {
					t.Errorf("tick %d relative RMS: OpenMPT %.3f tracker %.3f", tick, want, got)
				}
			}
		})
	}
}
