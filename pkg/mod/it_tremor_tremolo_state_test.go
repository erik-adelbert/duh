package mod

import (
	"fmt"
	"testing"
)

func TestITTremorTremoloLoader(t *testing.T) {
	for _, tt := range []struct {
		effect, internal byte
	}{{9, 0x22}, {18, 0x23}} {
		for arg := 0; arg <= 255; arg++ {
			for _, mptm := range []bool{false, true} {
				p, err := unpackITPatternMode([]byte{0x81, 8, tt.effect, byte(arg), 0, 0x81, 0x80, 0}, 2, mptm)
				if err != nil {
					t.Fatal(err)
				}
				for row := range p.Rows {
					cmd := p.Rows[row][0]
					if cmd.EffectCmd != tt.internal || cmd.EffectArg != byte(arg) {
						t.Fatalf("mptm=%t effect %d arg %02X row %d: %+v", mptm, tt.effect, arg, row, cmd)
					}
				}
			}
		}
	}
}

func TestITTremorTremoloMemory(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy}}, referenceRate, 1)
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x23, EffectArg: 0x45}
			VolumeEffectSystem(w)
			w.PendingCmds[0].EffectArg = 0x80
			VolumeEffectSystem(w)
			w.PendingCmds[0].EffectArg = 0x03
			VolumeEffectSystem(w)
			if w.TrackMemories[0].itTremolo != 0x83 || w.TrackMemories[1].itTremolo != 0 {
				t.Fatal("tremolo memory or track isolation is incorrect")
			}
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x22, EffectArg: 0x34}
			VolumeEffectSystem(w)
			want := uint8(0x23)
			if legacy {
				want = 0x34
			}
			w.PendingCmds[0].EffectArg = 0
			VolumeEffectSystem(w)
			if w.TrackMemories[0].itTremorParam != want || w.TrackMemories[1].itTremorParam != 0 {
				t.Fatal("tremor memory or track isolation is incorrect")
			}
		})
	}
}
