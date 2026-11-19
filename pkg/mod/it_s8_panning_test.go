package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITS8Panning(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for pan := range byte(16) {
			for _, before := range []bool{false, true} {
				t.Run(fmt.Sprintf("legacy=%v/pan=%d/before=%v", legacy, pan, before), func(t *testing.T) {
					b := testIT()
					b[50] = 1
					b[256+18] |= 0x10
					binary.LittleEndian.PutUint32(b[256+56:], 3840)
					rows := []byte{0x81, 11, 60, 1, 19, 0x80 | pan, 0, 0}
					if before {
						rows = []byte{0x81, 8, 19, 0x80 | pan, 0, 0x81, 3, 60, 1, 0}
					}
					packed := append(rows, make([]byte, 62)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					ref, actual := referencePCM(t, b)
					balance := func(pcm []float32) float64 {
						l, r := channelRMS(pcm, 50, 70, 0), channelRMS(pcm, 50, 70, 1)
						if l+r < 1e-6 {
							t.Fatal("silent comparison")
						}
						return (r - l) / (r + l)
					}
					want, got := balance(ref), balance(actual)
					if math.Abs(got-want) > .015 {
						t.Errorf("balance: OpenMPT %.4f, tracker %.4f", want, got)
					}
				})
			}
		}
	}
}

func TestOpenMPTITS8SurroundAndMemory(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"surround then pan", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0x81, 8, 19, 0x8c, 0}},
		{"pan then surround", []byte{0x81, 11, 60, 1, 19, 0x8c, 0, 0x81, 8, 19, 0x91, 0}},
		{"special memory", []byte{0x81, 11, 60, 1, 19, 0x8c, 0, 0x81, 8, 24, 32, 0, 0x81, 8, 19, 0, 0}},
	}

	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", tt.name, legacy), func(t *testing.T) {
				b := testIT()
				b[50] = 1
				b[256+18] |= 0x10
				binary.LittleEndian.PutUint32(b[256+56:], 3840)
				packed := append(append([]byte(nil), tt.rows...), make([]byte, 62)...)
				binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
				copy(b[344:], packed)
				if legacy {
					binary.LittleEndian.PutUint16(b[40:], 0x217)
					binary.LittleEndian.PutUint16(b[42:], 0x200)
				}
				ref, actual := referencePCM(t, b)
				ratio := func(pcm []float32) float64 {
					var ll, rr, lr float64
					for i := 70 * 48; i < 90*48; i++ {
						l, r := float64(pcm[2*i]), float64(pcm[2*i+1])
						ll, rr, lr = ll+l*l, rr+r*r, lr+l*r
					}
					if ll+rr < 1e-8 {
						t.Fatal("silent comparison")
					}
					return (rr - ll) / (rr + ll) * math.Copysign(1, lr)
				}
				want, got := ratio(ref), ratio(actual)
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .04 {
					t.Errorf("stereo balance: OpenMPT %.4f, tracker %.4f", want, got)
				}
			})
		}
	}
}

func TestITS8PanningState(t *testing.T) {
	b := testITInstrument()
	b[len(b)-554+304] = 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x8c}
	VolumeEffectSystem(w)
	want := float32((12*256+8)/15)/128 - 1
	if w.TrackCursors[0].Panning != want || w.TrackMemories[0].ITLastSpecial != 0x8c {
		t.Fatal("S8C did not update track panning and command memory")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x80}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Panning != want {
		t.Fatal("S80 changed panning after tick zero")
	}
	w.Clock.Tick = 0
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Panning != -1 || w.VoiceAmps[old].Panning != want {
		t.Fatal("S80 did not isolate the foreground voice")
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x8c}
	VolumeEffectSystem(w)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x08, EffectArg: 32}
	VolumeEffectSystem(w)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15}
	VolumeEffectSystem(w)
	if w.VoiceAmps[current].Panning != want {
		t.Fatal("S00 did not recall S8C")
	}
}
