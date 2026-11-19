package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITPanningSlide(t *testing.T) {
	for _, arg := range []byte{0x01, 0x20, 0xf2, 0x2f, 0x23, 0x0f, 0xf0, 0xff} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("P%02X/legacy=%t", arg, legacy), func(t *testing.T) {
				rows := []byte{0x81, 11, 60, 1, 16, arg, 0, 0x81, 8, 16, 0, 0}
				b := testITPitchFixture(rows, 2, true, legacy)
				ref, actual := referencePCMWithChunks(t, b, "2", 120, 256)
				for tick := range 6 {
					start := tick*20 + 19
					refL, refR := channelRMS(ref, start, start+1, 0), channelRMS(ref, start, start+1, 1)
					gotL, gotR := channelRMS(actual, start, start+1, 0), channelRMS(actual, start, start+1, 1)
					want, got := (refR-refL)/(refR+refL), (gotR-gotL)/(gotR+gotL)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
						t.Errorf("tick %d balance: OpenMPT %.3f, tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITPanningSlideState(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 2}, referenceRate, 1)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x1d, EffectArg: 1}
	VolumeEffectSystem(w)
	if w.TrackMemories[0].itPanSlide != 1 || w.TrackMemories[1].itPanSlide != 0 {
		t.Fatal("panning slide memory was not retained per track")
	}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if got := w.TrackCursors[0].Panning; got != 1.0/32 {
		t.Fatalf("P01 without a voice: pan %g, want 1/32", got)
	}
	w.PendingCmds[0].EffectArg = 0
	VolumeEffectSystem(w)
	if got := w.TrackCursors[0].Panning; got != 2.0/32 {
		t.Fatalf("P00 did not recall slide: pan %g", got)
	}
	p, err := unpackITPattern([]byte{0x81, 8, 16, 1, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if p.Rows[row][0].EffectCmd != 0x1d || p.Rows[row][0].EffectArg != 1 {
			t.Fatalf("row %d: unexpected command %+v", row, p.Rows[row][0])
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 16, 1, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}
