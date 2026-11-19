package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITArpeggio(t *testing.T) {
	tests := []struct {
		name     string
		rows     []byte
		speed    byte
		baseTick int
	}{
		{"J0C", []byte{0x81, 11, 60, 1, 10, 0x0c, 0}, 6, 0},
		{"J47", []byte{0x81, 11, 60, 1, 10, 0x47, 0}, 6, 0},
		{"JFF", []byte{0x81, 11, 60, 1, 10, 0xff, 0}, 6, 0},
		{"memory", []byte{0x81, 11, 60, 1, 10, 0x47, 0, 0x81, 8, 10, 0, 0}, 2, 0},
		{"memory before note", []byte{0x81, 8, 10, 0x47, 0, 0x81, 11, 60, 1, 10, 0, 0}, 2, 2},
		{"row reset", []byte{0x81, 11, 60, 1, 10, 0x47, 0, 0x81, 8, 10, 0, 0}, 4, 0},
		{"pitch slide before arpeggio", []byte{0x81, 11, 60, 1, 6, 32, 0, 0x81, 8, 10, 0x47, 0}, 2, 0},
		{"volume portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 10, 0x47, 0}, 2, 0},
	}

	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%v/linear=%v", tt.name, legacy, linear), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, tt.speed, linear, legacy)
					ref, actual := referencePCMForDuration(t, b, "2", 240)
					refBase, actualBase := referenceTickFrequency(t, ref, tt.baseTick), referenceTickFrequency(t, actual, tt.baseTick)
					for tick := tt.baseTick; tick < 12; tick++ {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .003 {
							t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestOpenMPTITArpeggioMappedInstrument(t *testing.T) {
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 10, 0x47, 0}
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%v/linear=%v", legacy, linear), func(t *testing.T) {
				fixture := testITPitchFixture(rows, 2, linear, legacy)
				b := testITInstrument()
				copy(b[40:44], fixture[40:44])
				b[44] |= fixture[44] & 8
				b[50] = 2
				copy(b[336:512], fixture[336:512])
				copy(b[512:512+8192], fixture[512:512+8192])
				binary.LittleEndian.PutUint32(b[256+56:], 3840)
				h := b[len(b)-554:]
				h[304] = 0
				h[64+60*2], h[64+72*2] = 64, 77
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referenceTickFrequency(t, ref, 1), referenceTickFrequency(t, actual, 1)
				for tick := 2; tick < 6; tick++ {
					want := referenceTickFrequency(t, ref, tick) / refBase
					got := referenceTickFrequency(t, actual, tick) / actualBase
					if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .003 {
						t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITArpeggioMemoryAndPastVoice(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			b := testITInstrument()
			b[len(b)-554+304] = 0
			loaded, err := loadIT(b)
			if err != nil {
				t.Fatal(err)
			}
			loaded.Assets.NumTracks = 2
			loaded.Assets.ITCompatibility.LegacyModPlug = legacy
			w := NewTracker(loaded.Assets, 48000, 256)
			w.PendingCmds[0] = ModCommand{EffectCmd: 0x10, EffectArg: 0x47}
			PitchEffectSystem(w)
			want := uint8(0x47)
			if legacy {
				want = 0
			}
			if w.TrackMemories[0].LastArpeggio != want || w.TrackMemories[1].LastArpeggio != 0 {
				t.Fatal("arpeggio memory without voice is incorrect or leaked across tracks")
			}
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x10, EffectArg: 0x47}
			VoiceAllocationSystem(w)
			w.Clock.Tick = 1
			PitchEffectSystem(w)
			old := w.TrackCursors[0].ActiveVoice
			if w.VoicePitches[old].VibratoOffset == 0 {
				t.Fatal("arpeggio did not change foreground pitch")
			}
			w.Clock.Tick = 0
			w.PendingCmds[0] = ModCommand{Note: 62}
			VoiceAllocationSystem(w)
			if !w.VoiceLives[old].ITPastNote || w.VoicePitches[old].VibratoOffset != 0 {
				t.Fatal("past voice retained a temporary arpeggio offset")
			}
		})
	}
}

func TestOpenMPTITArpeggioTunings(t *testing.T) {
	for _, tt := range []struct {
		note   byte
		c5rate uint32
	}{
		{48, 11025},
		{60, 8363},
		{72, 6000},
	} {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("note=%d/rate=%d/legacy=%v/linear=%v", tt.note, tt.c5rate, legacy, linear), func(t *testing.T) {
					rows := []byte{0x81, 11, tt.note, 1, 10, 0x47, 0}
					b := testITPitchFixture(rows, 6, linear, legacy)
					binary.LittleEndian.PutUint32(b[256+60:], tt.c5rate)
					ref, actual := referencePCMWithInterpolation(t, b, "2")
					refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
					for tick := 1; tick < 6; tick++ {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .003 {
							t.Errorf("tick %d pitch ratio: OpenMPT %.5f, tracker %.5f", tick, want, got)
						}
					}

				})
			}

		}
	}
}

func TestITArpeggioPackedReuse(t *testing.T) {
	pattern, err := unpackITPattern([]byte{0x81, 8, 10, 0x47, 0, 0x81, 0x80, 0, 0x81, 8, 10, 0, 0}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, arg := range []uint8{0x47, 0x47, 0} {
		if cmd := pattern.Rows[row][0]; cmd.EffectCmd != 0x10 || cmd.EffectArg != arg {
			t.Errorf("row %d: %+v", row, cmd)
		}
	}
}
