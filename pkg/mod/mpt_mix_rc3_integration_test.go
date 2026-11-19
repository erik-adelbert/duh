package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestLoadMPTMRC3Mix(t *testing.T) {
	b := mptmPlaybackFixture()
	b[bytesIndex(t, b, ".MMP")+6] = 3
	loaded, err := loadMPTM(b)
	if err != nil {
		t.Fatal(err)
	}
	if profile := loaded.Assets.ITCompatibility.mptMixProfile; profile == nil || !profile.softPanning {
		t.Fatalf("RC3 profile not loaded: %+v", profile)
	}
}

func TestOpenMPTRC3StereoAndSurround(t *testing.T) {
	for _, stereo := range []bool{false, true} {
		for _, pan := range []byte{0, 16, 32, 48, 64, 100} {
			t.Run(fmt.Sprintf("stereo=%t/pan=%d", stereo, pan), func(t *testing.T) {
				b, assets := mptMixFixture(t, false, false, 48, 48, testOPLPatch)
				b[bytesIndex(t, b, ".MMP")+6] = 3
				profile, err := projectMPTMixProfile(3, 48, 48)
				if err != nil {
					t.Fatal(err)
				}
				assets.ITCompatibility.mptMixProfile = &profile
				b[64] = pan
				if pan == 100 {
					assets.ITSurround[0] = true
				} else {
					assets.Panning[0] = float32(pan)/32 - 1
				}
				if stereo {
					properties := bytesIndex(t, b, "STPM")
					data := append([]byte(nil), b[512:512+4096*2]...)
					right := make([]byte, len(data))
					assets.Samples[0].RightData = make([]float32, 4096)
					for i := range assets.Samples[0].RightData {
						value := int16(binary.LittleEndian.Uint16(data[i*2:])) / 2
						binary.LittleEndian.PutUint16(right[i*2:], uint16(value))
						assets.Samples[0].RightData[i] = float32(value) / 32768
					}
					samples := append(data, right...)
					tail := append([]byte(nil), b[properties:]...)
					b = append(b[:properties], samples...)
					b = append(b, tail...)
					b[256+18] |= 4
					binary.LittleEndian.PutUint32(b[256+72:], uint32(properties))
					oldOffset := binary.LittleEndian.Uint32(b[len(b)-4:])
					binary.LittleEndian.PutUint32(b[len(b)-4:], oldOffset+uint32(len(samples)))
				}
				ref, actual := referenceTransition(t, b), renderMPTMixFixture(t, assets, 5760)
				for ch := range 2 {
					want, got := channelRMS(ref, 40, 100, ch), channelRMS(actual, 40, 100, ch)
					if want < .00001 {
						if got > .00001 {
							t.Errorf("channel %d: expected silence, got %g", ch, got)
						}
					} else if math.Abs(got/want-1) > .035 {
						t.Errorf("channel %d: OpenMPT %g tracker %g", ch, want, got)
					}
				}
				if pan == 100 {
					for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
						var cross float64
						for i := 1920 * 2; i < 4800*2; i += 2 {
							cross += float64(pcm[i]) * float64(pcm[i+1])
						}
						if cross >= 0 {
							t.Errorf("%s surround channels are not phase-inverted: %g", name, cross)
						}
					}
				}
			})
		}
	}
}

func TestOpenMPTRC3MixVolumes(t *testing.T) {
	for _, source := range []string{"pcm", "opl", "mixed"} {
		for _, tt := range []struct {
			sample, plugin uint32
			global         byte
		}{{32, 64, 128}, {64, 32, 128}, {48, 48, 64}, {0, 64, 128}, {64, 0, 128}} {
			t.Run(fmt.Sprintf("%s/sample=%d/plugin=%d/global=%d", source, tt.sample, tt.plugin, tt.global), func(t *testing.T) {
				b, assets := mptMixGlobalFixture(t, source == "opl", source == "mixed", tt.sample, tt.plugin, testOPLPatch, tt.global, nil)
				b[bytesIndex(t, b, ".MMP")+6] = 3
				profile, err := projectMPTMixProfile(3, tt.sample, tt.plugin)
				if err != nil {
					t.Fatal(err)
				}
				assets.ITCompatibility.mptMixProfile = &profile
				ref, actual := referenceTransition(t, b), renderMPTMixFixture(t, assets, 5760)
				for ch := range 2 {
					want, got := channelRMS(ref, 40, 100, ch), channelRMS(actual, 40, 100, ch)
					if want < .00001 {
						if got > .00001 {
							t.Errorf("channel %d: expected silence, got %g", ch, got)
						}
					} else if math.Abs(got/want-1) > .035 {
						t.Errorf("channel %d: OpenMPT %g tracker %g", ch, want, got)
					}
				}
			})
		}
	}
}
