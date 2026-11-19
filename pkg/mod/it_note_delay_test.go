package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITNoteDelay(t *testing.T) {
	tests := []struct {
		name        string
		rows        []byte
		speed       byte
		legacy      bool
		beforeStart int
		afterStart  int
	}{
		{"SD5", []byte{0x81, 11, 60, 1, 19, 0xd5, 0}, 6, false, 85, 105},
		{"legacy SD5", []byte{0x81, 11, 60, 1, 19, 0xd5, 0}, 6, true, 85, 105},
		{"SD0", []byte{0x81, 11, 60, 1, 19, 0xd0, 0}, 6, false, 5, 25},
		{"S00 recalls SD1", []byte{0x81, 8, 19, 0xd1, 0, 0x81, 11, 60, 1, 19, 0, 0}, 2, false, 45, 65},
		{"legacy S00 recalls SD1", []byte{0x81, 8, 19, 0xd1, 0, 0x81, 11, 60, 1, 19, 0, 0}, 2, true, 45, 65},
		{"out of range retains instrument", []byte{0x81, 11, 60, 1, 19, 0xd3, 0, 0x81, 1, 60, 0}, 2, false, 25, 45},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testITPitchFixture(tt.rows, tt.speed, true, tt.legacy)
			ref, actual := referencePCMWithInterpolation(t, b, "2")
			for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
				before := rmsWindow(pcm, tt.beforeStart, tt.beforeStart+8)
				after := rmsWindow(pcm, tt.afterStart, tt.afterStart+8)
				if before > 1e-5 || after < .001 {
					t.Errorf("%s: before %.6f after %.6f", name, before, after)
				}
			}
		})
	}
}

func TestOpenMPTITLegacySD0(t *testing.T) {
	b := testITPitchFixture([]byte{0x81, 11, 60, 1, 19, 0xd0, 0}, 6, true, true)
	ref, actual := referencePCMWithInterpolation(t, b, "2")
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		if rmsWindow(pcm, 8, 18) < .001 {
			t.Errorf("%s: SD0 did not start on tick zero", name)
		}
	}
}

func TestOpenMPTITLegacyOutOfRangeDelay(t *testing.T) {
	b := testITPitchFixture([]byte{0x81, 11, 60, 1, 19, 0xd3, 0, 0x81, 1, 60, 0}, 2, true, true)
	ref, actual := referencePCMWithInterpolation(t, b, "2")
	for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
		if rmsWindow(pcm, 45, 55) > 1e-5 {
			t.Errorf("%s: out-of-range note retained instrument", name)
		}
	}
}

func TestITNoteDelayVolumeSlideTiming(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1,
		Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.Clock.Speed = 6
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1, HasVolume: true, Volume: 96, EffectCmd: 0x15, EffectArg: 0xd3}
	for tick := 0; tick <= 4; tick++ {
		w.Clock.Tick = tick
		VoiceAllocationSystem(w)
		VolumeEffectSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		if tick < 3 && (v != old || w.VoiceAmps[v].Volume != 1) {
			t.Fatalf("tick %d changed the old note", tick)
		}
		if tick == 3 && w.VoiceAmps[v].Volume != 1 {
			t.Fatal("slide applied on delayed note's first tick")
		}
		if tick == 4 && math.Abs(float64(w.VoiceAmps[v].Volume-63.0/64)) > 1e-6 {
			t.Fatal("slide did not apply after delayed note's first tick")
		}
	}
}

func TestITNoteDelaySpecialMemoryAndPackedReuse(t *testing.T) {
	pattern, err := unpackITPattern([]byte{0x81, 8, 19, 0xd5, 0, 0x81, 0x80, 0, 0x81, 8, 19, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, arg := range []uint8{0xd5, 0xd5, 0} {
		if cmd := pattern.Rows[row][0]; cmd.EffectCmd != 0x15 || cmd.EffectArg != arg {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
	b := testIT()
	binary.LittleEndian.PutUint16(b[40:], 0x214)
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.Clock.Speed = 6
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0xd5}
	VolumeEffectSystem(w)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x15}
	for tick := 0; tick <= 5; tick++ {
		w.Clock.Tick = tick
		VoiceAllocationSystem(w)
		if got := w.TrackCursors[0].ActiveVoice; (got != -1) != (tick == 5) {
			t.Fatalf("S00 recalled SD5 on tick %d: voice %d", tick, got)
		}
	}
}
