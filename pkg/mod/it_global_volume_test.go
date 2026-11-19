package mod

import (
	"math"
	"testing"
)

func TestOpenMPTITGlobalVolume(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
		base int
	}{
		{"set and silence", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 22, 64, 0, 0x81, 8, 22, 0, 0}, 36},
		{"out of range", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 22, 64, 0, 0x81, 8, 22, 129, 0}, 36},
		{"maximum out of range", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 22, 64, 0, 0x81, 8, 22, 255, 0}, 36},
		{"before note", []byte{0x81, 8, 22, 64, 0, 0x81, 3, 60, 1, 0, 0x81, 8, 22, 128, 0}, 76},
		{"other channel", []byte{0x81, 3, 60, 1, 0, 0x82, 8, 22, 64, 0, 0x82, 8, 22, 128, 0}, 36},
		{"multiple commands", []byte{0x81, 3, 60, 1, 0, 0x81, 8, 22, 32, 0x82, 8, 22, 96, 0}, 36},
		{"volume column", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 86, 22, 64, 0}, 36},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			name := tt.name + "/IT"
			if legacy {
				name = tt.name + "/ModPlug"
			}
			t.Run(name, func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				b[256+19] = 32
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase := channelRMS(ref, tt.base, tt.base+3, 0)
				actualBase := channelRMS(actual, tt.base, tt.base+3, 0)
				if refBase < 1e-6 || actualBase < 1e-6 {
					t.Fatal("silent comparison baseline")
				}
				for _, ms := range []int{76, 116} {
					want := channelRMS(ref, ms, ms+3, 0) / refBase
					got := channelRMS(actual, ms, ms+3, 0) / actualBase
					if math.IsNaN(want) || math.IsNaN(got) || math.Abs(want-got) > .035 {
						t.Errorf("%dms: relative volume OpenMPT %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}

func TestITGlobalVolumeState(t *testing.T) {
	assets := SongAssets{
		IT: true, NumTracks: 2, ITGlobalVolume: 128, ITMixGain: 1,
		ITChannelVolume: [MaxTracks]uint8{64, 64}, Samples: []SampleAsset{{ITGlobalVolume: 64}},
	}
	w, other := NewTracker(assets, 48000, 1), NewTracker(assets, 48000, 1)
	for voice, track := range []TrackID{0, 1, 0} {
		w.VoiceLives[voice] = VoiceLifecycle{Active: true, ParentTrack: track, FadeOutVol: 1, ITPastNote: voice == 2, ITPastChannelVolume: 64}
		w.VoiceAmps[voice].Volume = 1
	}
	w.PendingCmds[1] = ModCommand{EffectCmd: 0x17, EffectArg: 64}
	VolumeEffectSystem(w)
	for voice := range 3 {
		if got := voiceVolume(w, voice, 0); got != .5 {
			t.Errorf("voice %d after V40 = %g, want .5", voice, got)
		}
	}
	if w.ITGlobalVolume != 64 || other.ITGlobalVolume != 128 || w.Assets.ITGlobalVolume != 128 {
		t.Fatal("V40 changed assets or another world")
	}
	w.PendingCmds[1].EffectArg = 0
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 64 {
		t.Fatal("V00 changed volume after tick zero")
	}
	w.Clock.Tick = 0
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 0 {
		t.Fatal("V00 did not set volume to zero")
	}
	for _, arg := range []uint8{129, 255} {
		w.PendingCmds[1].EffectArg = arg
		VolumeEffectSystem(w)
		if w.ITGlobalVolume != 0 {
			t.Fatalf("out-of-range V%02X changed global volume", arg)
		}
	}
	w.Assets.ITCompatibility.LegacyModPlug = true
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 128 {
		t.Fatal("legacy out-of-range Vxx did not clamp to 128")
	}
	w.Assets.ITCompatibility.LegacyModPlug = false
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x17, EffectArg: 32}
	w.PendingCmds[1].EffectArg = 96
	VolumeEffectSystem(w)
	if w.ITGlobalVolume != 96 {
		t.Fatalf("simultaneous global volume commands: got %d", w.ITGlobalVolume)
	}
}

func TestITGlobalVolumePackedReuse(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 8, 22, 128, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if cmd := p.Rows[row][0]; cmd.EffectCmd != 0x17 || cmd.EffectArg != 128 {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
}
