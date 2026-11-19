package mod

import (
	"fmt"
	"testing"
)

func TestOpenMPTITNoteCut(t *testing.T) {
	type expectation struct {
		hasBefore bool
		cut       bool
	}
	tests := []struct {
		name            string
		rows            []byte
		speed           byte
		beforeStart     int
		afterStart      int
		regular, legacy expectation
	}{
		{"SC3", []byte{0x81, 11, 60, 1, 19, 0xc3, 0}, 6, 45, 100, expectation{true, true}, expectation{true, true}},
		{"SC0", []byte{0x81, 11, 60, 1, 19, 0xc0, 0}, 6, 5, 80, expectation{true, true}, expectation{false, true}},
		{"out of range", []byte{0x81, 11, 60, 1, 19, 0xc6, 0}, 6, 45, 90, expectation{true, false}, expectation{true, false}},
		{"speed one", []byte{0x81, 11, 60, 1, 19, 0xc0, 0}, 1, 5, 80, expectation{true, false}, expectation{false, true}},
		{"S00 recalls SC1", []byte{0x81, 8, 19, 0xc1, 0, 0x81, 11, 60, 1, 19, 0, 0}, 2, 45, 100, expectation{true, true}, expectation{true, true}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, tt.speed, true, legacy)
				ref, actual := referencePCMWithChunks(t, b, "2", 120, 128)
				for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
					before := rmsWindow(pcm, tt.beforeStart, tt.beforeStart+8)
					after := rmsWindow(pcm, tt.afterStart, tt.afterStart+8)
					expected := tt.regular
					if legacy {
						expected = tt.legacy
					}
					if (before > .001) != expected.hasBefore || (after < .001) != expected.cut {
						t.Errorf("%s: before %.6f after %.6f; want sound before = %v, cut = %v", name, before, after, expected.hasBefore, expected.cut)
					}
				}
			})
		}
	}
}

func TestOpenMPTITNoteCutVolumeRestore(t *testing.T) {
	for _, tt := range []struct {
		name  string
		rows  []byte
		start int
	}{
		{"volume only", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 4, 64, 0}, 65},
		{"sample only", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 2, 1, 0}, 65},
		{"new note", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 1, 60, 0}, 65},
		{"new note with volume", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 5, 60, 64, 0}, 65},
		{"new note with sample", []byte{0x81, 11, 60, 1, 19, 0xc1, 0, 0x81, 3, 60, 1, 0}, 65},
		{"SC0 then sample", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0xc0, 0, 0x81, 2, 1, 0}, 105},
		{"SC0 then note", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 19, 0xc0, 0, 0x81, 1, 60, 0}, 105},
		{"note cut then sample", []byte{0x81, 3, 60, 1, 0, 0x81, 1, 254, 0, 0x81, 2, 1, 0}, 105},
	} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%v", tt.name, legacy), func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				ref, actual := referencePCMWithChunks(t, b, "2", 160, 128)
				want := rmsWindow(ref, tt.start, tt.start+10) > .001
				got := rmsWindow(actual, tt.start, tt.start+10) > .001
				if got != want {
					t.Errorf("restored sound: OpenMPT %v, tracker %v", want, got)
				}
			})
		}
	}
}

func TestITLegacySC0Upgrade(t *testing.T) {
	rows := []byte{0x81, 8, 19, 0x8c, 0, 0x81, 11, 60, 1, 19, 0xc0, 0, 0x81, 8, 19, 0, 0}
	for _, legacy := range []bool{false, true} {
		b := testITPitchFixture(rows, 2, true, legacy)
		loaded, err := loadIT(b)
		if err != nil {
			t.Fatal(err)
		}
		cmd := loaded.Assets.Patterns[0].Rows[1][0]
		if legacy {
			if cmd.Note != 254 || cmd.EffectCmd != 0 {
				t.Fatalf("legacy SC0 was not upgraded to note cut: %+v", cmd)
			}
		} else if cmd.Note != 61 || cmd.EffectCmd != 0x15 || cmd.EffectArg != 0xc0 {
			t.Fatalf("regular IT SC0 changed during loading: %+v", cmd)
		}
		w := NewTracker(loaded.Assets, 48000, 256)
		w.PendingCmds[0] = loaded.Assets.Patterns[0].Rows[0][0]
		VolumeEffectSystem(w)
		w.PendingCmds[0] = cmd
		VoiceAllocationSystem(w)
		VolumeEffectSystem(w)
		want := uint8(0xc0)
		if legacy {
			want = 0x8c
		}
		if got := w.TrackMemories[0].ITLastSpecial; got != want {
			t.Fatalf("legacy=%v special memory = %02x, want %02x", legacy, got, want)
		}
	}
}

func TestITNoteCutStateAndSpecialMemory(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			b := testITInstrument()
			b[len(b)-554+304] = 0
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.ITCompatibility.LegacyModPlug = legacy
			w := NewTracker(loaded.Assets, 48000, 256)
			w.Clock.Speed = 6
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			old := w.TrackCursors[0].ActiveVoice
			w.PendingCmds[0] = ModCommand{Note: 62, EffectCmd: 0x15, EffectArg: 0xc3}
			VoiceAllocationSystem(w)
			current := w.TrackCursors[0].ActiveVoice
			for tick := 0; tick <= 3; tick++ {
				w.Clock.Tick = tick
				VolumeEffectSystem(w)
				if tick < 3 && (!w.VoiceLives[current].Active || w.VoiceAmps[current].Volume == 0) {
					t.Fatalf("cut too early at tick %d", tick)
				}
			}
			if !w.VoiceLives[old].Active || !w.VoiceLives[old].ITPastNote || w.VoiceAmps[old].Volume == 0 {
				t.Fatal("note cut changed the past voice")
			}
			if legacy {
				if !w.VoiceLives[current].Active || w.VoiceAmps[current].Volume != 0 {
					t.Fatal("legacy note cut did not mute foreground voice")
				}
			} else if w.VoiceLives[current].Active {
				t.Fatal("IT note cut did not stop foreground voice")
			}
			if w.TrackMemories[0].ITLastSpecial != 0xc3 {
				t.Fatal("SC3 not saved in special-effect memory")
			}
		})
	}
}
