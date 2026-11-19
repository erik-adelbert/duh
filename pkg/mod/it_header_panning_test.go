package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestITHeaderChannelPanning(t *testing.T) {
	for raw := 0; raw <= 255; raw++ {
		t.Run(fmt.Sprintf("%02x", raw), func(t *testing.T) {
			b := testIT()
			b[64], b[128] = byte(raw), 17
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			pan := byte(raw) & 127
			wantPan, wantVolume := float32(0), byte(17)
			wantMute, wantSurround := raw&128 != 0, pan == 100
			if pan <= 64 {
				wantPan = float32(pan)/32 - 1
			}
			if raw == 255 {
				wantVolume, wantMute = 64, false
			}
			a := loaded.Assets
			if a.Panning[0] != wantPan || a.ITMuted[0] != wantMute ||
				a.ITSurround[0] != wantSurround || a.ITChannelVolume[0] != wantVolume {
				t.Fatalf("pan=%g mute=%t surround=%t volume=%d; want %g %t %t %d",
					a.Panning[0], a.ITMuted[0], a.ITSurround[0], a.ITChannelVolume[0],
					wantPan, wantMute, wantSurround, wantVolume)
			}
		})
	}
}

func TestMPTMHeaderChannelPanning(t *testing.T) {
	for _, raw := range []byte{89, 0x80 | 89, 255} {
		t.Run(fmt.Sprintf("%02x", raw), func(t *testing.T) {
			b := mptmPlaybackFixture()
			b[64], b[128] = raw, 17
			loaded, err := loadMPTM(b)
			if err != nil {
				t.Fatal(err)
			}
			wantVolume, wantMute := byte(17), raw&128 != 0
			if raw == 255 {
				wantVolume, wantMute = 64, false
			}
			a := loaded.Assets
			if a.Panning[0] != 0 || a.ITSurround[0] || a.ITMuted[0] != wantMute ||
				a.ITChannelVolume[0] != wantVolume {
				t.Fatalf("unexpected channel settings: pan=%g mute=%t surround=%t volume=%d",
					a.Panning[0], a.ITMuted[0], a.ITSurround[0], a.ITChannelVolume[0])
			}
		})
	}
}

func TestOpenMPTITHeaderChannelPanning(t *testing.T) {
	for _, raw := range []byte{0, 32, 64, 65, 89, 99, 101, 126, 127, 0x80 | 89, 255} {
		t.Run(fmt.Sprintf("%02x", raw), func(t *testing.T) {
			b := testIT()
			b[64], b[128] = raw, 0
			if raw != 255 {
				b[128] = 64
			}
			ref, actual := referencePCM(t, b)
			for _, pcm := range [][]float32{ref, actual} {
				level := rmsWindow(pcm, 20, 40)
				if raw&128 != 0 && raw != 255 {
					if level > 0.00001 {
						t.Fatalf("muted channel produced audio: %g", level)
					}
					continue
				}
				left, right := channelRMS(pcm, 20, 40, 0), channelRMS(pcm, 20, 40, 1)
				if left+right < 0.001 {
					t.Fatal("channel is silent")
				}
				want := 0.0
				if raw <= 64 {
					want = float64(raw)/32 - 1
				}
				if balance := (right - left) / (right + left); math.Abs(balance-want) > .02 {
					t.Fatalf("balance = %g, want %g", balance, want)
				}
			}
		})
	}
}
