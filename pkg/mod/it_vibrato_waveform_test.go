package mod

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITVibratoWaveforms(t *testing.T) {
	for _, waveform := range []byte{0, 1, 2, 4, 5, 6, 8, 9, 10, 12, 13, 14} {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				for _, linear := range []bool{false, true} {
					t.Run(fmt.Sprintf("S3%X/legacy=%t/old=%t/linear=%t", waveform, legacy, old, linear), func(t *testing.T) {
						rows := []byte{0x81, 11, 60, 1, 19, 0x30 | waveform, 0,
							0x81, 8, 8, 0x8f, 0, 0x81, 11, 60, 1, 8, 0, 0,
							0x81, 8, 19, 0, 0, 0x81, 8, 21, 0, 0}
						b := testITPitchFixture(rows, 3, linear, legacy)
						if old {
							b[44] |= 0x10
						}
						ref, actual := referencePCMWithChunks(t, b, "2", 300, 256)
						for tick := range 15 {
							want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
							if math.IsNaN(got) || math.Abs(got/want-1) > .0015 {
								t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
							}
						}
					})
				}
			}
		}
	}
}

func TestITVibratoWaveformCommands(t *testing.T) {
	for arg := byte(0x30); arg <= 0x3f; arg++ {
		pattern, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0, 0x81, 8, 19, 0, 0}, 2, false)
		if err != nil {
			t.Fatalf("S%02X: %v", arg, err)
		}
		if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err != nil {
			t.Fatalf("MPTM S%02X: %v", arg, err)
		}
		for _, legacy := range []bool{false, true} {
			w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy}}, 48000, 256)
			w.PendingCmds[0] = pattern.Rows[0][0]
			PitchEffectSystem(w)
			VolumeEffectSystem(w)
			want := arg & 15
			if legacy {
				want &= 7
			} else if want >= 4 {
				want = 0
			}
			if w.TrackMemories[0].itVibratoWaveform != want || w.TrackMemories[1].itVibratoWaveform != 0 {
				t.Fatalf("S%02X legacy=%t: waveform=%d", arg, legacy, w.TrackMemories[0].itVibratoWaveform)
			}
			w.TrackMemories[0].itVibratoWaveform = 15
			w.PendingCmds[0] = pattern.Rows[1][0]
			PitchEffectSystem(w)
			VolumeEffectSystem(w)
			if w.TrackMemories[0].itVibratoWaveform != want {
				t.Fatalf("S%02X then S00 legacy=%t: waveform=%d", arg, legacy, w.TrackMemories[0].itVibratoWaveform)
			}
		}
	}
}

func TestITLegacyRandomVibratoRejected(t *testing.T) {
	for _, arg := range []byte{0x33, 0x37, 0x3b, 0x3f} {
		b := testITPitchFixture([]byte{0x81, 8, 19, arg, 0}, 3, true, true)
		if _, err := loadIT(b); err == nil || !strings.Contains(err.Error(), "legacy random vibrato waveform") {
			t.Fatalf("S%02X: got %v", arg, err)
		}
	}
}

func TestOpenMPTITVibratoWaveformInteractions(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"volume vibrato on selection row", []byte{0x81, 3, 60, 1, 0, 0x81, 12, 208, 19, 0x31, 0, 0x81, 4, 208, 0}},
		{"combined Kxx", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 8, 19, 0x31, 0, 0x81, 8, 11, 0, 0}},
		{"selection on retrigger", []byte{0x81, 11, 60, 1, 19, 0x35, 0, 0x81, 8, 8, 0x8f, 0, 0x81, 11, 60, 1, 19, 0x31, 0, 0x81, 8, 8, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/old=%t", tt.name, legacy, old), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 3, true, legacy)
					if old {
						b[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 240, 256)
					for tick := range 12 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.Abs(got/want-1) > .0015 {
							t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITRandomVibratoWaveform(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1,
		Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x15, EffectArg: 0x33}
	VoiceAllocationSystem(w)
	PitchEffectSystem(w)
	VolumeEffectSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x04, EffectArg: 0x8f}
	seen := map[int]bool{}
	for tick := range 128 {
		w.Clock.Tick = tick
		PitchEffectSystem(w)
		delta := w.VoicePitches[voice].itVibratoDelta
		if delta < -60 || delta > 59 {
			t.Fatalf("random vibrato delta out of range: %d", delta)
		}
		seen[delta] = true
	}
	if len(seen) < 20 {
		t.Fatalf("random vibrato produced only %d distinct values", len(seen))
	}
}

func TestITLegacyModulationWaveformProfile(t *testing.T) {
	tests := []struct {
		name   string
		compat ITCompatibility
		want   bool
	}{
		{name: "modern IT"},
		{name: "legacy IT", compat: ITCompatibility{LegacyModPlug: true}, want: true},
		{name: "MPTM precise", compat: ITCompatibility{
			mptMixProfile: &mptMixProfile{}, itPanbrelloProfile: &itPanbrelloProfile{preciseWaveform: true},
		}},
		{name: "MPTM legacy", compat: ITCompatibility{
			mptMixProfile: &mptMixProfile{}, itPanbrelloProfile: &itPanbrelloProfile{},
		}, want: true},
		{name: "non-MPTM profile", compat: ITCompatibility{itPanbrelloProfile: &itPanbrelloProfile{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := Tracker{Assets: SongAssets{ITCompatibility: tt.compat}}
			if got := w.itLegacyModulationWaveform(); got != tt.want {
				t.Fatalf("legacy waveform = %t, want %t", got, tt.want)
			}
		})
	}
}
