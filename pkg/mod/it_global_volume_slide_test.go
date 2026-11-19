package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITGlobalVolumeSlide(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"fine up and memory", []byte{0x81, 11, 60, 1, 23, 0x1f, 0, 0x81, 8, 23, 0, 0}},
		{"fine down", []byte{0x81, 11, 60, 1, 23, 0xf3, 0, 0x81, 8, 23, 0, 0}},
		{"regular up", []byte{0x81, 11, 60, 1, 23, 0x20, 0, 0x81, 8, 23, 0, 0}},
		{"regular down", []byte{0x81, 11, 60, 1, 23, 0x03, 0, 0x81, 8, 23, 0, 0}},
		{"mixed nibbles", []byte{0x81, 11, 60, 1, 23, 0x23, 0, 0x81, 8, 23, 0, 0}},
		{"cross channel memory", []byte{0x81, 11, 60, 1, 23, 0x1f, 0, 0x82, 8, 23, 0, 0}},
		{"silence and clamp", []byte{0x81, 11, 60, 1, 23, 0xff, 0, 0x81, 8, 23, 0xf8, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				b[48] = 64
				baseline := testITPitchFixture([]byte{0x81, 3, 60, 1, 0}, 2, true, legacy)
				baseline[48] = 64
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
				for _, ms := range []int{15, 35, 55, 75, 95, 115} {
					want := channelRMS(ref, ms, ms+5, 0) / channelRMS(refBase, ms, ms+5, 0)
					got := channelRMS(actual, ms, ms+5, 0) / channelRMS(actualBase, ms, ms+5, 0)
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
						t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}

func TestITGlobalVolumeSlideState(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITGlobalVolume: 127}, referenceRate, 1)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x1c, EffectArg: 0x1f}
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 128 {
		t.Fatal("fine up did not clamp at 128")
	}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 128 {
		t.Fatal("fine slide ran after tick zero")
	}
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{}
	w.PendingCmds[1] = ModCommand{EffectCmd: 0x1c}
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 128 || w.TrackMemories[1].itGlobalVolSlide != 0 {
		t.Fatal("global slide memory leaked between channels")
	}
}

func TestITGlobalVolumeSlideTickAndMemory(t *testing.T) {
	tests := []struct {
		arg   uint8
		tick0 uint8
		tick1 uint8
	}{
		{0x1f, 65, 65},
		{0xf3, 61, 61},
		{0x20, 64, 66},
		{0x03, 64, 61},
		{0x23, 64, 64},
		{0xff, 79, 79},
	}
	for _, tt := range tests {
		w := NewTracker(SongAssets{IT: true, NumTracks: 1, ITGlobalVolume: 64}, referenceRate, 1)
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x1c, EffectArg: tt.arg}
		VolumeEffectSystem(w)
		if got := w.ITGlobalVolume; got != tt.tick0 {
			t.Errorf("W%02X tick 0: got %d, want %d", tt.arg, got, tt.tick0)
		}
		w.Clock.Tick = 1
		VolumeEffectSystem(w)
		if got := w.ITGlobalVolume; got != tt.tick1 {
			t.Errorf("W%02X tick 1: got %d, want %d", tt.arg, got, tt.tick1)
		}
		w.Clock.Tick = 0
		w.PendingCmds[0].EffectArg = 0
		VolumeEffectSystem(w)
		want := max(0, min(128, int(tt.tick1)+int(tt.tick0)-64))
		if got := int(w.ITGlobalVolume); got != want {
			t.Errorf("W%02X followed by W00: got %d, want %d", tt.arg, got, want)
		}
	}
}

func TestITGlobalVolumeSlideLoader(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 8, 23, 0x1f, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x1c || cmd.EffectArg != 0x1f {
			t.Errorf("row %d: unexpected command %+v", row, cmd)
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 23, 0x1f, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}
