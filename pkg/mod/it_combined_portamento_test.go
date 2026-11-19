package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITCombinedPortamentoVolumeSlide(t *testing.T) {
	for _, arg := range []byte{0x02, 0x20, 0xf2, 0x2f, 0x23} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("L%02X/legacy=%t", arg, legacy), func(t *testing.T) {
				rows := []byte{0x81, 15, 60, 1, 48, 7, 0x10, 0,
					0x81, 9, 64, 12, arg, 0, 0x81, 8, 12, 0, 0}
				baseline := []byte{0x81, 15, 60, 1, 48, 7, 0x10, 0,
					0x81, 9, 64, 7, 0, 0, 0x81, 8, 7, 0, 0}
				b := testITPitchFixture(rows, 2, true, legacy)
				base := testITPitchFixture(baseline, 2, true, legacy)
				ref, actual := referencePCMWithChunks(t, b, "2", 120, 256)
				refBase, actualBase := referencePCMWithChunks(t, base, "2", 120, 256)
				for tick := range 6 {
					ms := tick*20 + 15
					want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
					got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .04 {
						t.Errorf("tick %d: volume ratio OpenMPT %.3f tracker %.3f", tick, want, got)
					}
					wantPitch, gotPitch := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(gotPitch/wantPitch-1) > .002 {
						t.Errorf("tick %d: frequency OpenMPT %.3f tracker %.3f", tick, wantPitch, gotPitch)
					}
				}
			})
		}
	}
}

func TestITCombinedPortamentoVolumeSlideLoader(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 8, 12, 2, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x1f || cmd.EffectArg != 2 || !itHasTonePortamento(cmd) {
			t.Fatalf("row %d: %+v", row, cmd)
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 12, 2, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := unpackITPattern([]byte{0x81, 12, 195, 12, 2, 0}, 1); err == nil {
		t.Fatal("unverified volume-column portamento combined with Lxx accepted")
	}
}
