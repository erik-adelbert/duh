package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITVibrato(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"H82", []byte{0x81, 11, 60, 1, 8, 0x82, 0}},
		{"deep", []byte{0x81, 11, 60, 1, 8, 0x3f, 0}},
		{"fast and deep", []byte{0x81, 11, 60, 1, 8, 0xff, 0}},
		{"memory and partial nibbles", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 8, 8, 0x04, 0, 0x81, 8, 8, 0x30, 0, 0x81, 8, 8, 0, 0}},
		{"interrupt and resume", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0, 0x81, 8, 8, 0, 0}},
		{"retrigger note", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 11, 60, 1, 8, 0, 0}},
		{"memory before note", []byte{0x81, 8, 8, 0x82, 0, 0x81, 11, 60, 1, 8, 0, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				for _, old := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/legacy=%v/linear=%v/old=%v", tt.name, legacy, linear, old), func(t *testing.T) {
						b := testITPitchFixture(tt.rows, 6, linear, legacy)
						if old {
							b[44] |= 0x10
						}
						ref, actual := referencePCMForDuration(t, b, "2", 480)
						first := 0
						if tt.name == "memory before note" {
							first = 6
						}
						for tick := first; tick < 24; tick++ {
							want := referenceTickFrequency(t, ref, tick)
							got := referenceTickFrequency(t, actual, tick)
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

func TestITVibratoMemoryWithoutVoice(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 2}, 48000, 16)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x04, EffectArg: 0x82}
	PitchEffectSystem(w)
	w.PendingCmds[0].EffectArg = 0x03
	PitchEffectSystem(w)
	if w.TrackMemories[0].LastVibrato != 0x83 || w.TrackMemories[1].LastVibrato != 0 {
		t.Fatal("vibrato memory must persist without a voice and remain isolated by track")
	}
}

func TestITVibratoPastVoice(t *testing.T) {
	b := testITInstrument()
	b[len(b)-554+304] = 0
	loaded, err := loadIT(b)
	if err != nil {
		t.Fatal(err)
	}
	w := NewTracker(loaded.Assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x04, EffectArg: 0x8f}
	VoiceAllocationSystem(w)
	PitchEffectSystem(w)
	old := w.TrackCursors[0].ActiveVoice
	if w.VoicePitches[old].VibratoOffset == 0 {
		t.Fatal("vibrato did not change foreground pitch")
	}
	w.PendingCmds[0] = ModCommand{Note: 62}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if old == current || !w.VoiceLives[old].ITPastNote || w.VoicePitches[old].VibratoOffset != 0 {
		t.Fatal("past voice retained a temporary vibrato offset")
	}
	PitchEffectSystem(w)
	if w.VoicePitches[current].VibratoOffset != 0 {
		t.Fatal("new note inherited a temporary vibrato offset")
	}
}

func TestOpenMPTITVibratoSpeedOne(t *testing.T) {
	rows := []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 8, 8, 0, 0, 0x81, 8, 8, 0, 0}
	for _, legacy := range []bool{false, true} {
		for _, old := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%v/old=%v", legacy, old), func(t *testing.T) {
				b := testITPitchFixture(rows, 1, true, legacy)
				if old {
					b[44] |= 0x10
				}
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				for tick := range 6 {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.IsNaN(got) || math.Abs(got/want-1) > .0015 {
						t.Errorf("tick %d: OpenMPT %.3f Hz, tracker %.3f Hz", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITFineVibratoLoader(t *testing.T) {
	for arg := 0; arg <= 255; arg++ {
		p, err := unpackITPattern([]byte{0x81, 8, 21, byte(arg), 0, 0x81, 0x80, 0}, 2)
		if err != nil {
			t.Fatalf("U%02X: %v", arg, err)
		}
		for row := range p.Rows {
			cmd := p.Rows[row][0]
			if cmd.EffectCmd != 0x21 || cmd.EffectArg != byte(arg) {
				t.Fatalf("U%02X row %d: %+v", arg, row, cmd)
			}
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 21, 0x12, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMPTITFineVibrato(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"fine and memory", []byte{0x81, 11, 60, 1, 21, 0x8f, 0, 0x81, 8, 21, 0, 0, 0x81, 8, 8, 0, 0}},
		{"normal to fine", []byte{0x81, 11, 60, 1, 8, 0x82, 0, 0x81, 8, 21, 0x8f, 0, 0x81, 8, 8, 0, 0}},
		{"fine after normal", []byte{0x81, 11, 60, 1, 8, 0x8f, 0, 0x81, 8, 21, 0, 0, 0x81, 8, 21, 3, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/old=%t", tt.name, legacy, old), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 6, true, legacy)
					if old {
						b[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 360, 256)
					for tick := range 18 {
						want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
						if math.IsNaN(got) || math.Abs(got/want-1) > .0015 {
							t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTITCombinedVibratoVolumeSlide(t *testing.T) {
	tests := []struct {
		name string
		arg  byte
	}{
		{"down", 0x02}, {"up", 0x20}, {"fine down", 0xf2},
		{"fine up", 0x2f}, {"mixed", 0x23},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, old := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/old=%t", tt.name, legacy, old), func(t *testing.T) {
					rows := []byte{0x81, 15, 60, 1, 48, 8, 0x82, 0,
						0x81, 8, 11, tt.arg, 0, 0x81, 8, 11, 0, 0, 0x81, 8, 4, 0, 0}
					baselineRows := []byte{0x81, 15, 60, 1, 48, 8, 0x82, 0,
						0x81, 8, 8, 0, 0, 0x81, 8, 8, 0, 0, 0x81, 8, 4, 0, 0}
					b := testITPitchFixture(rows, 2, true, legacy)
					baseline := testITPitchFixture(baselineRows, 2, true, legacy)
					if old {
						b[44] |= 0x10
						baseline[44] |= 0x10
					}
					ref, actual := referencePCMWithChunks(t, b, "2", 160, 256)
					refBase, actualBase := referencePCMWithChunks(t, baseline, "2", 160, 256)
					for tick := range 8 {
						start := tick*20 + 15
						want := channelRMS(ref, start, start+5, 0) / channelRMS(refBase, start, start+5, 0)
						got := channelRMS(actual, start, start+5, 0) / channelRMS(actualBase, start, start+5, 0)
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .04 {
							t.Errorf("tick %d volume ratio: OpenMPT %.3f, tracker %.3f", tick, want, got)
						}
						if tick < 6 {
							wantPitch := referenceTickFrequency(t, ref, tick)
							gotPitch := referenceTickFrequency(t, actual, tick)
							if math.Abs(gotPitch/wantPitch-1) > .002 {
								t.Errorf("tick %d pitch: OpenMPT %.3f, tracker %.3f", tick, wantPitch, gotPitch)
							}
						}
					}
				})
			}
		}
	}
}

func TestITCombinedVibratoVolumeSlideMemory(t *testing.T) {
	w := NewTracker(SongAssets{IT: true, NumTracks: 2}, referenceRate, 1)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x11, EffectArg: 2}
	VolumeEffectSystem(w)
	if w.TrackMemories[0].LastVolSlide != 2 || w.TrackMemories[1].LastVolSlide != 0 {
		t.Fatal("Kxx memory not retained per track without a voice")
	}
	p, err := unpackITPattern([]byte{0x81, 8, 11, 2, 0, 0x81, 0x80, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for row := range p.Rows {
		if p.Rows[row][0].EffectCmd != 0x11 || p.Rows[row][0].EffectArg != 2 {
			t.Fatalf("row %d: unexpected command %+v", row, p.Rows[row][0])
		}
	}
	if _, err := unpackITPatternMode([]byte{0x81, 8, 11, 2, 0}, 1, true); err != nil {
		t.Fatal(err)
	}
}

func TestOpenMPTITRandomVibratoRange(t *testing.T) {
	rows := []byte{0x81, 11, 60, 1, 19, 0x33, 0, 0x81, 8, 8, 0x8f, 0,
		0x81, 8, 8, 0, 0, 0x81, 8, 8, 0, 0}
	for _, old := range []bool{false, true} {
		t.Run(fmt.Sprintf("old=%t", old), func(t *testing.T) {
			b := testITPitchFixture(rows, 6, true, false)
			if old {
				b[44] |= 0x10
			}
			ref, actual := referencePCMWithChunks(t, b, "2", 480, 256)
			for name, pcm := range map[string][]float32{"OpenMPT": ref, "tracker": actual} {
				base := referenceTickFrequency(t, pcm, 0)
				above, below := false, false
				for tick := 6; tick < 24; tick++ {
					ratio := referenceTickFrequency(t, pcm, tick) / base
					if math.IsNaN(ratio) || ratio < .88 || ratio > 1.14 {
						t.Fatalf("%s tick %d: unexpected frequency ratio %v", name, tick, ratio)
					}
					above = above || ratio > 1.005
					below = below || ratio < .995
				}
				if !above || !below {
					t.Fatalf("%s random vibrato did not vary in both directions", name)
				}
			}
		})
	}
}
