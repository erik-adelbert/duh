package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestOpenMPTITVolumeSlideDown(t *testing.T) {
	tests := []struct {
		name   string
		rows   []byte
		legacy bool
	}{
		{"slide and memory", []byte{0x81, 7, 60, 1, 98, 0, 0x81, 4, 95, 0}, false},
		{"legacy slide and memory", []byte{0x81, 7, 60, 1, 98, 0, 0x81, 4, 95, 0}, true},
		{"maximum and clamp", []byte{0x81, 7, 60, 1, 104, 0, 0x81, 4, 104, 0}, false},
		{"initial zero", []byte{0x81, 7, 60, 1, 95, 0}, false},
		{"memory before note", []byte{0x81, 4, 99, 0, 0x81, 7, 60, 1, 95, 0}, false},
		{"slide with surround", []byte{0x81, 15, 60, 1, 98, 19, 0x91, 0, 0x81, 12, 95, 19, 0x90, 0}, false},
		{"slide with panning", []byte{0x81, 15, 60, 1, 98, 24, 128, 0, 0x81, 12, 95, 24, 128, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := testIT()
			b[50] = 2
			b[256+18] |= 0x10
			binary.LittleEndian.PutUint32(b[256+56:], 3840)
			if tt.legacy {
				binary.LittleEndian.PutUint16(b[40:], 0x217)
				binary.LittleEndian.PutUint16(b[42:], 0x200)
			}
			packed := append(append([]byte(nil), tt.rows...), make([]byte, 64)...)
			binary.LittleEndian.PutUint16(b[336:], uint16(len(packed)))
			copy(b[344:], packed)
			ref, actual := referencePCM(t, b)
			start := 36
			if tt.name == "memory before note" {
				start = 76
			}
			refBase := channelRMS(ref, 116, 119, 0)
			actualBase := channelRMS(actual, 116, 119, 0)
			if refBase < 1e-6 || actualBase < 1e-6 {
				t.Fatal("silent comparison baseline")
			}
			for ms := start; ms <= 116; ms += 20 {
				want := channelRMS(ref, ms, ms+3, 0) / refBase
				got := channelRMS(actual, ms, ms+3, 0) / actualBase
				if math.Abs(want-got) > .035 {
					t.Errorf("%dms: relative volume OpenMPT %.3f, tracker %.3f", ms, want, got)
				}
			}
		})
	}
}

func TestOpenMPTITVolumeSlideUp(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"slide by one", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 86, 0}},
		{"slide and memory", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 88, 0, 0x81, 4, 85, 0}},
		{"maximum and clamp", []byte{0x81, 7, 60, 1, 52, 0, 0x81, 4, 94, 0, 0x81, 4, 94, 0}},
		{"initial zero", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 85, 0}},
		{"shared direction memory", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 99, 0, 0x81, 4, 85, 0}},
		{"down recalls up", []byte{0x81, 7, 60, 1, 32, 0, 0x81, 4, 89, 0, 0x81, 4, 95, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			name := tt.name + "/IT"
			if legacy {
				name = tt.name + "/ModPlug"
			}
			t.Run(name, func(t *testing.T) {
				b := testITPitchFixture(tt.rows, 2, true, legacy)
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase := channelRMS(ref, 36, 39, 0)
				actualBase := channelRMS(actual, 36, 39, 0)
				if refBase < 1e-6 || actualBase < 1e-6 {
					t.Fatal("silent comparison baseline")
				}
				for ms := 56; ms <= 116; ms += 20 {
					want := channelRMS(ref, ms, ms+3, 0) / refBase
					got := channelRMS(actual, ms, ms+3, 0) / actualBase
					if math.Abs(want-got) > .035 {
						t.Errorf("%dms: relative volume OpenMPT %.3f, tracker %.3f", ms, want, got)
					}
				}
			})
		}
	}
}

func TestITVolumeSlideUpState(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		w := NewTracker(SongAssets{
			IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy},
			Samples: []SampleAsset{{Data: make([]float32, 1024), Volume: 32, C5Speed: 48000}},
		}, 48000, 256)
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, HasVolume: true, Volume: 89}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		VolumeEffectSystem(w)
		if w.VoiceAmps[v].Volume != 0.5 || w.TrackMemories[0].ITVolSlide != 4 || w.TrackMemories[1].ITVolSlide != 0 {
			t.Fatalf("legacy=%v: initial volume or per-track memory incorrect", legacy)
		}
		w.Clock.Tick = 1
		VolumeEffectSystem(w)
		if got := w.VoiceAmps[v].Volume; got != 36.0/64 {
			t.Fatalf("legacy=%v: slide volume = %g", legacy, got)
		}
		w.PendingCmds[0].Volume = 95
		VolumeEffectSystem(w)
		if got := w.VoiceAmps[v].Volume; got != 0.5 {
			t.Fatalf("legacy=%v: opposite direction memory volume = %g", legacy, got)
		}
		w.VoiceAmps[v].Volume = 63.0 / 64
		w.PendingCmds[0].Volume = 85
		VolumeEffectSystem(w)
		if got := w.VoiceAmps[v].Volume; got != 1 {
			t.Fatalf("legacy=%v: slide did not clamp at one: %g", legacy, got)
		}
		if legacy && w.TrackMemories[0].LastVolSlide != 0x40 || !legacy && w.TrackMemories[0].LastVolSlide != 0 {
			t.Fatalf("legacy=%v: effect-column memory = %x", legacy, w.TrackMemories[0].LastVolSlide)
		}
	}
}

func TestITVolumeSlideDownState(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2,
		Samples: []SampleAsset{{Data: make([]float32, 1024), Volume: 64, C5Speed: 48000}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 99}
	VolumeEffectSystem(w)
	if w.TrackMemories[0].ITVolSlide != 4 || w.TrackMemories[1].ITVolSlide != 0 {
		t.Fatal("volume-column memory was not retained per track")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, HasVolume: true, Volume: 95, EffectCmd: 0x15, EffectArg: 0x91}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	VolumeEffectSystem(w)
	if w.VoiceAmps[v].Volume != 1 || !w.VoiceAmps[v].Surround {
		t.Fatal("slide ran on tick zero or prevented effect-column command")
	}
	w.Clock.Tick = 1
	VolumeEffectSystem(w)
	if got := w.VoiceAmps[v].Volume; got != 60.0/64 {
		t.Fatalf("recalled slide volume = %g", got)
	}
	w.VoiceAmps[v].Volume = 2.0 / 64
	VolumeEffectSystem(w)
	if got := w.VoiceAmps[v].Volume; got != 0 {
		t.Fatalf("slide did not clamp at zero: %g", got)
	}
	if w.TrackMemories[0].LastVolSlide != 0 {
		t.Fatal("IT volume-column slide polluted effect-column memory")
	}
}

func TestITVolumeSlidePackedReuseAndValidation(t *testing.T) {
	p, err := unpackITPattern([]byte{0x81, 4, 86, 0, 0x81, 0x40, 0, 0x81, 4, 95, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []uint8{86, 86, 95} {
		if got := p.Rows[i][0]; !got.HasVolume || got.Volume != want {
			t.Errorf("row %d: %+v, want volume %d", i, got, want)
		}
	}
	for _, volume := range []byte{125, 127} {
		if _, err := unpackITPattern([]byte{0x81, 4, volume, 0}, 1); err == nil {
			t.Errorf("unsupported volume command %d accepted", volume)
		}
	}
}
