package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestITS7AOpenMPT(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"with note", []byte{0x81, 11, 60, 1, 19, 0x7a, 0}},
		{"later row", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0x7a, 0}},
		{"special memory", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0x7a, 0, 0x81, 8, 19, 0, 0}},
	}
	for _, tt := range tests {
		for _, enabled := range []bool{false, true} {
			for _, legacy := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/enabled=%t/legacy=%t", tt.name, enabled, legacy), func(t *testing.T) {
					b := testITInstrument()
					b[len(b)-554+304] = 0
					h := b[len(b)-554:]
					setTestEnvelope(h, 386, 1, []int8{-24, 24}, []uint16{0, 12})
					if !enabled {
						h[386] &^= 1
					}
					if legacy {
						binary.LittleEndian.PutUint16(b[40:], 0x217)
						binary.LittleEndian.PutUint16(b[42:], 0x200)
					}
					packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
					binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
					copy(b[344:], packed)
					ref, actual := referencePCMWithChunks(t, b, "2", 240, 256)
					for tick := range 12 {
						if tick == 6 && !enabled && tt.name != "with note" {
							continue
						}
						start := tick*20 + 10
						wantL, wantR := channelRMS(ref, start, start+5, 0), channelRMS(ref, start, start+5, 1)
						gotL, gotR := channelRMS(actual, start, start+5, 0), channelRMS(actual, start, start+5, 1)
						want, got := wantL/(wantL+wantR), gotL/(gotL+gotR)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .035 {
							t.Errorf("tick %d left balance: OpenMPT %.3f, tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITS7AVoiceState(t *testing.T) {
	b := testITInstrument()
	h := b[len(b)-554:]
	setTestEnvelope(h, 386, 0, []int8{-24, 24}, []uint16{0, 12})
	packed := append([]byte{0x81, 11, 60, 1, 19, 0x7a, 0}, make([]byte, 64)...)
	binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
	copy(b[344:], packed)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, referenceRate, 256)
	w.PendingCmds[0] = loaded.Assets.Patterns[0].Rows[0][0]
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	VolumeEffectSystem(w)
	VoiceLifecycleSystem(w)
	if w.VoiceLives[v].itPanEnvelopeControl != itEnvelopeEnabledOverride || w.VoiceLives[v].ITPanEnvelopeValue != -24 || w.VoiceLives[v].ITPanEnvelopeTick != 1 {
		t.Fatalf("S7A did not start disabled envelope on tick zero: %+v", w.VoiceLives[v])
	}
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	next := w.TrackCursors[0].ActiveVoice
	if w.VoiceLives[next].itPanEnvelopeControl != itEnvelopeDefault || w.VoiceLives[v].itPanEnvelopeControl != itEnvelopeEnabledOverride {
		t.Fatal("pan envelope override was not isolated to its voice")
	}
}

func TestITS7ALoader(t *testing.T) {
	for arg := byte(0x70); arg <= 0x7c; arg++ {
		p, err := unpackITPattern([]byte{0x81, 8, 19, arg, 0, 0x81, 0x80, 0}, 2)
		if err != nil {
			t.Fatalf("S%02X: %v", arg, err)
		}
		for row := range p.Rows {
			if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x15 || cmd.EffectArg != arg {
				t.Fatalf("S%02X row %d: %+v", arg, row, cmd)
			}
		}
	}
	for _, arg := range []byte{0x7d, 0x7e, 0x7f} {
		if _, err := unpackITPattern([]byte{0x81, 8, 19, arg, 0}, 1); err == nil {
			t.Errorf("unsupported S%02X accepted", arg)
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 19, 0x7a, 0}, 1, true); err != nil {
		t.Fatalf("supported MPTM S7A rejected: %v", err)
	}
}
