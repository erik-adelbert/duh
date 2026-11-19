package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITPanningEffect(t *testing.T) {
	for _, tt := range []struct {
		name       string
		pan        byte
		legacy     bool
		beforeNote bool
		instrument bool
		volPan     bool
	}{
		{"left", 0, false, false, false, false},
		{"quarter", 64, false, false, false, false},
		{"center", 128, false, false, false, false},
		{"three quarters", 192, false, false, false, false},
		{"right", 255, false, false, false, false},
		{"legacy left", 0, true, false, false, false},
		{"legacy right", 255, true, false, false, false},
		{"before note", 64, false, true, false, false},
		{"instrument pitch pan override", 192, false, false, true, false},
		{"volume pan conflict", 192, false, false, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			if tt.instrument {
				b = testITInstrument()
				h := b[len(b)-554:]
				h[304] = 0
				h[22], h[23] = 16, 48
			}
			b[50] = 1
			b[256+18] |= 0x10
			binary.LittleEndian.PutUint32(b[256+56:], 3840)
			var packed []byte
			if tt.beforeNote {
				packed = []byte{0x81, 8, 24, tt.pan, 0, 0x81, 3, 60, 1, 0}
			} else if tt.volPan {
				packed = []byte{0x81, 0x0f, 60, 1, 128, 24, tt.pan, 0, 0}
			} else {
				packed = []byte{0x81, 0x0b, 60, 1, 24, tt.pan, 0, 0}
			}
			packed = append(packed, make([]byte, 62)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			ref, actual := referencePCM(t, b)
			balance := func(pcm []float32) float64 {
				left, right := channelRMS(pcm, 50, 70, 0), channelRMS(pcm, 50, 70, 1)
				if left+right < 1e-6 {
					t.Fatal("silent comparison window")
				}
				return (right - left) / (right + left)
			}
			if want, got := balance(ref), balance(actual); math.Abs(want-got) > .015 {
				t.Errorf("balance: OpenMPT %.5f, tracker %.5f", want, got)
			}
		})
	}
}

func TestITPanningEffectMemoryAndVoices(t *testing.T) {
	b := testITInstrument()
	b[len(b)-554+304] = 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x08, EffectArg: 64}
	VolumeEffectSystem(w)
	if w.TrackCursors[0].Panning != -.5 {
		t.Fatal("panning without an active voice was not retained")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	if w.VoiceAmps[old].Panning != -.5 {
		t.Fatal("note did not inherit track panning")
	}
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	w.VoiceAmps[current].ITPanSwing = 7
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x08, EffectArg: 255}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Panning != -.5 {
		t.Fatal("effect applied after tick zero")
	}
	w.Clock.Tick = 0
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Panning != float32(127)/128 || w.TrackCursors[0].Panning != float32(127)/128 ||
		w.VoiceAmps[current].ITPanSwing != 0 || w.VoiceAmps[old].Panning != -.5 {
		t.Fatal("panning did not update only the foreground voice and track")
	}
}

func TestS3MPanningEffectRange(t *testing.T) {
	w := testWorld()
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	for _, tt := range []struct {
		arg  uint8
		want float32
	}{{0, -1}, {64, 0}, {128, 1}} {
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x08, EffectArg: tt.arg}
		VolumeEffectSystem(w)
		if got := w.VoiceAmps[v].Panning; got != tt.want {
			t.Fatalf("X%02X: got %g, want %g", tt.arg, got, tt.want)
		}

	}
}

func TestITPanningEffectReuseAndZero(t *testing.T) {
	packed := []byte{0x81, 8, 24, 192, 0, 0x81, 8, 24, 0, 0, 0x81, 0x80, 0}
	pattern, err := unpackITPattern(packed, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, want := range []uint8{192, 0, 0} {
		cmd := pattern.Rows[row][0]
		if cmd.EffectCmd != 0x08 || cmd.EffectArg != want {
			t.Fatalf("row %d: effect %+v, want X%02X", row, cmd, want)
		}
	}
}
