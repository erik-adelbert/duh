package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITChannelVolumeSlideTickAndMemory(t *testing.T) {
	tests := []struct {
		arg          uint8
		tick0, tick1 uint8
	}{
		{0x00, 32, 32}, {0x10, 32, 33}, {0x03, 32, 29},
		{0x1f, 33, 33}, {0xf3, 29, 29}, {0x23, 32, 32},
		{0xff, 47, 47}, {0xf0, 32, 47}, {0x0f, 32, 17},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("N%02X", tt.arg), func(t *testing.T) {
			w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITChannelVolume: [MaxTracks]uint8{32, 32}}, referenceRate, 1)
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x1e, EffectArg: tt.arg}
			VolumeEffectSystem(w)
			if got := w.ITChannelVolume[0]; got != tt.tick0 {
				t.Fatalf("tick 0: got %d, want %d", got, tt.tick0)
			}
			w.Clock.Tick = 1
			VolumeEffectSystem(w)
			if got := w.ITChannelVolume[0]; got != tt.tick1 {
				t.Fatalf("tick 1: got %d, want %d", got, tt.tick1)
			}
			w.Clock.Tick = 0
			w.PendingCmds[0].EffectArg = 0
			w.PendingCmds[1] = ModCommand{EffectCmd: 0x1e}
			VolumeEffectSystem(w)
			want := max(0, min(64, int(tt.tick1)+int(tt.tick0)-32))
			if int(w.ITChannelVolume[0]) != want || w.ITChannelVolume[1] != 32 || w.TrackMemories[1].itChannelVolSlide != 0 {
				t.Fatalf("N00 memory or channel isolation: volumes %v", w.ITChannelVolume[:2])
			}
		})
	}
	for _, tt := range []struct {
		initial, arg, tick, want uint8
	}{{63, 0x2f, 0, 64}, {1, 0xf2, 0, 0}, {63, 0x20, 1, 64}, {1, 0x02, 1, 0}} {
		w := NewTracker(SongAssets{IT: true, NumTracks: 1, ITChannelVolume: [MaxTracks]uint8{tt.initial}}, referenceRate, 1)
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x1e, EffectArg: tt.arg}
		w.Clock.Tick = int(tt.tick)
		VolumeEffectSystem(w)
		if got := w.ITChannelVolume[0]; got != tt.want {
			t.Errorf("N%02X from %d: got %d, want %d", tt.arg, tt.initial, got, tt.want)
		}
	}
}

func TestITChannelVolumeSlideLoader(t *testing.T) {
	for arg := 0; arg <= 255; arg++ {
		p, err := unpackITPattern([]byte{0x81, 8, 14, byte(arg), 0, 0x81, 0x80, 0}, 2)
		if err != nil {
			t.Fatalf("N%02X: %v", arg, err)
		}
		for row := range p.Rows {
			if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x1e || cmd.EffectArg != byte(arg) {
				t.Fatalf("N%02X row %d: %+v", arg, row, cmd)
			}
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 14, 0x10, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMPTITChannelVolumeSlide(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"regular up", []byte{0x81, 11, 60, 1, 14, 0x10, 0, 0x81, 8, 14, 0, 0}},
		{"regular down", []byte{0x81, 11, 60, 1, 14, 0x03, 0, 0x81, 8, 14, 0, 0}},
		{"fine up", []byte{0x81, 11, 60, 1, 14, 0x2f, 0, 0x81, 8, 14, 0, 0}},
		{"fine down", []byte{0x81, 11, 60, 1, 14, 0xf3, 0, 0x81, 8, 14, 0, 0}},
		{"mixed nibbles", []byte{0x81, 11, 60, 1, 14, 0x23, 0, 0x81, 8, 14, 0, 0}},
		{"ff", []byte{0x81, 11, 60, 1, 14, 0xff, 0, 0x81, 8, 14, 0, 0}},
		{"before note", []byte{0x81, 8, 14, 0xf8, 0, 0x81, 3, 60, 1, 0}},
		{"volume column", []byte{0x81, 15, 60, 1, 32, 14, 0x10, 0, 0x81, 12, 86, 14, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				b[128] = 32
				baseline := testITPitchFixture([]byte{0x81, 3, 60, 1, 0}, 2, true, legacy)
				baseline[128] = 32
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
				for _, ms := range []int{55, 75, 95, 115} {
					if tt.name == "before note" && ms == 55 {
						continue
					}
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

func TestOpenMPTITChannelVolumeSlidePastNote(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			fixture := func(arg byte) []byte {
				b := testITInstrument()
				b[50], b[128] = 2, 32
				h := b[len(b)-554:]
				h[17], h[304] = 1, 0
				binary.LittleEndian.PutUint32(b[256+60:], 3840)
				for i := range 4096 {
					binary.LittleEndian.PutUint16(b[512+i*2:], 12000)
				}
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 3, 60, 1, 0, 0x81, 8, 14, arg, 0}
				packed := append(rows, make([]byte, 64)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				return b
			}
			ref, actual := referencePCMWithChunks(t, fixture(0xf8), "1", 140, 256)
			refBase, actualBase := referencePCMWithChunks(t, fixture(0), "1", 140, 256)
			for _, ms := range []int{99, 119, 139} {
				want := channelRMS(ref, ms-1, ms, 0) / channelRMS(refBase, ms-1, ms, 0)
				got := channelRMS(actual, ms-1, ms, 0) / channelRMS(actualBase, ms-1, ms, 0)
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
					t.Errorf("%dms: OpenMPT ratio %.3f, tracker %.3f", ms, want, got)
				}
			}
		})
	}
}
