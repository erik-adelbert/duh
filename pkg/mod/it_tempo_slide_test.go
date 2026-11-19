package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITTempoSlideLoader(t *testing.T) {
	for arg := 0; arg <= 255; arg++ {
		p, err := unpackITPattern([]byte{0x81, 8, 20, byte(arg), 0, 0x81, 0x80, 0}, 2)
		if err != nil {
			t.Fatalf("T%02X: %v", arg, err)
		}
		for row := range p.Rows {
			cmd := p.Rows[row][0]
			if cmd.EffectCmd != 0x20 || cmd.EffectArg != byte(arg) {
				t.Fatalf("T%02X row %d: %+v", arg, row, cmd)
			}
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 20, 0x01, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}

func TestITTempoSlideState(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 2}, 48000, 1)
	w.Clock.Tempo = 125
	for _, tt := range []struct {
		arg  uint8
		tick int
		want float64
	}{
		{0x02, 0, 125}, {0x02, 1, 123}, {0x02, 2, 121},
		{0x00, 0, 121}, {0x00, 1, 119}, {0x13, 1, 122},
		{0x20, 0, 32}, {0x02, 1, 32}, {0xff, 0, 255}, {0x1f, 1, 255},
	} {
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x20, EffectArg: tt.arg}
		w.Clock.Tick = tt.tick
		GlobalFlowSystem(w)
		if w.Clock.Tempo != tt.want {
			t.Fatalf("T%02X tick %d: got %g, want %g", tt.arg, tt.tick, w.Clock.Tempo, tt.want)
		}
		if w.TrackMemories[1].itTempoSlide != 0 {
			t.Fatal("tempo memory leaked to another channel")
		}
	}
}

func TestOpenMPTITTempoSlide(t *testing.T) {
	for _, arg := range []byte{0x03, 0x14, 0x1f} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("T%02X/legacy=%t", arg, legacy), func(t *testing.T) {
				rows := []byte{0x81, 11, 60, 1, 20, arg, 0,
					0x81, 11, 60, 1, 20, 0, 0,
					0x81, 3, 60, 1, 0}
				b := testITPitchFixture(rows, 6, false, legacy)
				b[256+18] &^= 0x10
				binary.LittleEndian.PutUint32(b[256+48:], 700)
				binary.LittleEndian.PutUint32(b[256+60:], 48000)
				for i := range 700 {
					binary.LittleEndian.PutUint16(b[512+i*2:], 16000)
				}
				ref, actual := referencePCMWithChunks(t, b, "1", 360, 256)
				onsets := func(pcm []float32) []int {
					var found []int
					previous := false
					for i := 0; i < len(pcm)/2; i++ {
						active := math.Abs(float64(pcm[2*i])) > .001
						if active && !previous {
							found = append(found, i)
						}
						previous = active
					}
					return found
				}
				want, got := onsets(ref), onsets(actual)
				if len(want) < 3 || len(got) < 3 {
					t.Fatalf("insufficient note onsets: OpenMPT %v tracker %v", want, got)
				}
				for i := 1; i < 3; i++ {
					if delta := got[i] - want[i]; delta < -20 || delta > 20 {
						t.Errorf("onset %d: OpenMPT %d tracker %d frames", i, want[i], got[i])
					}
				}
			})
		}
	}
}
