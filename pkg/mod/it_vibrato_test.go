package mod

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITSampleVibrato(t *testing.T) {
	for _, tt := range []struct {
		name               string
		wave               byte
		legacy             bool
		rate, sweep, depth byte
	}{
		{"sine", 0, false, 32, 255, 32},
		{"ramp down", 1, false, 32, 255, 32},
		{"square", 2, false, 32, 255, 32},
		{"ramp up", 4, false, 32, 255, 32},
		{"saturated depth", 2, false, 32, 255, 2},
		{"legacy sine", 0, true, 32, 255, 32},
		{"legacy ramp down", 1, true, 32, 255, 32},
		{"legacy square", 2, true, 32, 255, 32},
		{"legacy ramp up", 4, true, 32, 255, 32},
		{"legacy zero rate", 2, true, 0, 255, 32},
		{"zero sweep", 0, false, 32, 0, 32},
		{"zero rate", 0, false, 0, 255, 32},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[256+18] |= 0x10
			binary.LittleEndian.PutUint32(b[256+56:], 3840)
			b[256+76], b[256+77], b[256+78], b[256+79] = tt.rate, tt.depth, tt.sweep, tt.wave
			for i := range 4096 {
				value := int16(math.Sin(float64(i)*2*math.Pi/8) * 16000)
				binary.LittleEndian.PutUint16(b[512+i*2:], uint16(value))
			}

			b[336+8+6], b[336+8+7], b[336+8+8] = 0, 0, 0
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			baseline := append([]byte(nil), b...)
			baseline[256+77] = 0
			refBase, actualBase := referencePCMWithInterpolation(t, baseline, "2")
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			for tick := range 6 {
				want := referenceTickFrequency(t, ref, tick) / referenceTickFrequency(t, refBase, tick)
				got := referenceTickFrequency(t, actual, tick) / referenceTickFrequency(t, actualBase, tick)
				if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .00025 {
					t.Errorf("tick %d frequency ratio: OpenMPT %.6f, tracker %.6f", tick, want, got)
				}
			}
		})
	}
}

func TestITSampleVibratoVoiceState(t *testing.T) {
	b := testITInstrument()
	b[256+76], b[256+77], b[256+78], b[256+79] = 32, 3, 255, 2
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	s := loaded.Assets.Samples[0]
	if s.ITVibratoRate != 32 || s.ITVibratoDepth != 3 || s.ITVibratoSweep != 255 || s.ITVibratoType != 2 {
		t.Fatalf("vibrato fields: %+v", s)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	for range 4 {
		VoiceKinematicsSystem(w)
	}
	if p := w.VoicePitches[old]; p.ITAutoVibDepth != 3*256 || p.ITAutoVibPos != 128 {
		t.Fatalf("vibrato phase or depth: %+v", p)
	}
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	next := w.TrackCursors[0].ActiveVoice
	if old == next || !w.VoiceLives[old].Active || w.VoicePitches[next].ITAutoVibDepth != 0 || w.VoicePitches[next].ITAutoVibPos != 0 {
		t.Fatal("new note did not reset its independent vibrato state")
	}
	VoiceKinematicsSystem(w)
	if w.VoicePitches[old].ITAutoVibPos != 160 || w.VoicePitches[next].ITAutoVibPos != 32 {
		t.Fatal("overlapping voices did not advance vibrato independently")
	}
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	if w.VoicePitches[next].ITAutoVibPos != 64 {
		t.Fatal("vibrato stopped on key-off")
	}
}

func TestITSampleVibratoRandomAndLegacyRejection(t *testing.T) {
	b := testIT()
	b[256+76], b[256+77], b[256+78], b[256+79] = 37, 8, 255, 3
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	makeWorld := func() *Tracker {
		w := NewTracker(loaded.Assets, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		return w
	}
	a, bWorld := makeWorld(), makeWorld()
	changed := false
	for range 24 {
		VoiceKinematicsSystem(a)
		VoiceKinematicsSystem(bWorld)
		x, y := a.VoicePitches[0].StepIncrement, bWorld.VoicePitches[0].StepIncrement
		if x != y || x < math.Pow(2, -8.0/768) || x > math.Pow(2, 8.0/768) {
			t.Fatalf("random vibrato: %v, %v", x, y)
		}
		changed = changed || x != 1
	}
	if !changed {
		t.Fatal("random vibrato never changed pitch")
	}
	binary.LittleEndian.PutUint16(b[40:], 0x217)
	binary.LittleEndian.PutUint16(b[42:], 0x200)
	if _, err := loadIT(b); err == nil || !strings.Contains(err.Error(), "unsupported legacy random auto-vibrato") {
		t.Fatalf("got error %v, want legacy random vibrato rejection", err)
	}
}
