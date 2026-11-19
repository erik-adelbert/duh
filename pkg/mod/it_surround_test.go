package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITSurround(t *testing.T) {
	tests := []struct {
		name      string
		rows      []byte
		header    bool
		legacy    bool
		samplePan bool
	}{
		{"S91 with note", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0}, false, false, false},
		{"S91 before note", []byte{0x81, 8, 19, 0x91, 0, 0x81, 3, 60, 1, 0}, false, false, false},
		{"S90 after S91", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0x81, 8, 19, 0x90, 0}, false, false, false},
		{"S00 repeats S91", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0x81, 8, 24, 128, 0, 0x81, 8, 19, 0, 0}, false, false, false},
		{"panning disables surround", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0x81, 8, 24, 128, 0}, false, false, false},
		{"header surround", []byte{0x81, 3, 60, 1, 0, 0}, true, false, false},
		{"sample pan clears surround", []byte{0x81, 8, 19, 0x91, 0, 0x81, 3, 60, 1, 0}, false, false, true},
		{"legacy sample pan retains surround", []byte{0x81, 8, 19, 0x91, 0, 0x81, 3, 60, 1, 0}, false, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[50] = 1
			b[256+18] |= 0x10
			binary.LittleEndian.PutUint32(b[256+56:], 3840)
			if tt.header {
				b[64] = 100
			}
			if tt.samplePan {
				b[256+47] = 128 | 48
			}
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			packed := append(append([]byte(nil), tt.rows...), make([]byte, 62)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			ref, actual := referencePCM(t, b)
			balance := func(pcm []float32) float64 {
				var ll, rr, lr float64
				for i := 50 * 48; i < 70*48; i++ {
					l, r := float64(pcm[2*i]), float64(pcm[2*i+1])
					ll, rr, lr = ll+l*l, rr+r*r, lr+l*r
				}
				if ll < 1e-8 {
					t.Fatal("silent comparison")
				}
				return lr / ll
			}
			want, got := balance(ref), balance(actual)
			if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .04 {
				t.Errorf("right/left ratio: OpenMPT %.3f, tracker %.3f", want, got)
			}
		})
	}
}

func TestITSurroundState(t *testing.T) {
	b := testITInstrument()
	b[len(b)-554+304] = 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x91}
	VolumeEffectSystem(w)
	if !w.TrackCursors[0].Surround || w.TrackCursors[0].Panning != 0 {
		t.Fatal("S91 before a note did not set track surround")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x90}
	VolumeEffectSystem(w)
	if old == current || !w.VoiceAmps[old].Surround || w.VoiceAmps[current].Surround || w.TrackCursors[0].Surround {
		t.Fatal("S90 did not isolate background voice surround")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x91}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Surround {
		t.Fatal("S91 executed after first tick")
	}
	w.Clock.Tick = 0
	VolumeEffectSystem(w)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x08, EffectArg: 128}
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Surround || w.TrackCursors[0].Surround {
		t.Fatal("X80 did not disable surround")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15}
	VolumeEffectSystem(w)
	if !w.VoiceAmps[current].Surround {
		t.Fatal("S00 did not repeat S91")
	}
}

func TestITSurroundEffectValidation(t *testing.T) {
	for _, arg := range []byte{0, 0x90, 0x91} {
		cmd, got, err := mapITEffect(19, arg)
		if err != nil || cmd != 0x15 || got != arg {
			t.Fatalf("S%02X: %d %d %v", arg, cmd, got, err)
		}
	}
	_, _, err := mapITEffect(19, 0x92)
	if err == nil || !strings.Contains(err.Error(), "unsupported IT effect S92") {
		t.Fatalf("unexpected error: %v", err)
	}
}
