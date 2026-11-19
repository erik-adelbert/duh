package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTRC3MixComparison(t *testing.T) {
	for _, source := range []string{"pcm", "opl", "mixed"} {
		for _, pan := range []uint8{0, 16, 32, 48, 64} {
			t.Run(fmt.Sprintf("%s/pan=%d", source, pan), func(t *testing.T) {
				b, assets := mptMixFixture(t, source == "opl", source == "mixed", 48, 48, testOPLPatch)
				b[bytesIndex(t, b, ".MMP")+6] = 3
				profile, err := projectMPTMixProfile(3, 48, 48)
				if err != nil {
					t.Fatal(err)
				}
				assets.ITCompatibility.mptMixProfile = &profile
				for ch := 0; ch < assets.NumTracks; ch++ {
					b[64+ch] = pan
					assets.Panning[ch] = float32(pan)/32 - 1
				}
				ref, actual := referenceTransition(t, b), renderMPTMixFixture(t, assets, 5760)
				for channel := range 2 {
					want, got := channelRMS(ref, 40, 100, channel), channelRMS(actual, 40, 100, channel)
					if want < .00001 {
						if got > .00001 {
							t.Errorf("channel %d: expected silence, got %.6f", channel, got)
						}
					} else if math.Abs(got/want-1) > .035 {
						t.Errorf("channel %d: OpenMPT %.6f tracker %.6f", channel, want, got)
					}
				}
			})
		}
	}
}
