package mod

import (
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITVolumePortamento(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"initial note", []byte{0x81, 7, 60, 1, 202, 0}},
		{"up", []byte{0x81, 3, 60, 1, 0, 0x81, 5, 72, 199, 0, 0x81, 4, 193, 0}},
		{"down", []byte{0x81, 3, 72, 1, 0, 0x81, 5, 60, 199, 0, 0x81, 4, 193, 0}},
		{"with instrument", []byte{0x81, 3, 60, 1, 0, 0x81, 7, 72, 1, 199, 0}},
		{"effect memory to volume", []byte{0x81, 3, 60, 1, 0, 0x81, 9, 72, 7, 32, 0, 0x81, 4, 193, 0}},
		{"volume memory to effect", []byte{0x81, 3, 60, 1, 0, 0x81, 5, 72, 198, 0, 0x81, 8, 7, 0, 0}},
		{"both tone portamentos", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 7, 8, 0}},
		{"effect pitch slide", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 6, 8, 0}},
		{"with note delay", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 19, 0xd1, 0}},
		{"memory before note", []byte{0x81, 4, 198, 0, 0x81, 3, 60, 1, 0, 0x81, 5, 72, 193, 0}},
		{"G00 with volume speed", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 7, 0, 0}},
		{"with vibrato", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 8, 0x82, 0}},
		{"with volume slide", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 4, 1, 0}},
		{"delay outside row", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 19, 0xd3, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, compatible := range []bool{false, true} {
				for _, linear := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/legacy=%v/compatible=%v/linear=%v", tt.name, legacy, compatible, linear), func(t *testing.T) {
						b := testITPitchFixture(tt.rows, 2, linear, legacy)
						if compatible {
							b[44] |= 0x20
						}
						ref, actual := referencePCMForDuration(t, b, "2", 160)
						baseTick := 1
						if tt.name == "memory before note" {
							baseTick = 3
						}
						refBase, actualBase := referenceTickFrequency(t, ref, baseTick), referenceTickFrequency(t, actual, baseTick)
						for tick := baseTick + 1; tick < 8; tick++ {
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
}

func TestOpenMPTITVolumePortamentoSpeeds(t *testing.T) {
	for speed := range byte(10) {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("speed=%d/legacy=%v", speed, legacy), func(t *testing.T) {
				rows := []byte{0x81, 3, 60, 1, 0, 0x81, 5, 72, 193 + speed, 0}
				b := testITPitchFixture(rows, 2, true, legacy)
				ref, actual := referencePCMWithInterpolation(t, b, "2")
				refBase, actualBase := referenceTickFrequency(t, ref, 1), referenceTickFrequency(t, actual, 1)
				want := referenceTickFrequency(t, ref, 3) / refBase
				got := referenceTickFrequency(t, actual, 3) / actualBase
				if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .003 {
					t.Errorf("pitch ratio: OpenMPT %.5f, tracker %.5f", want, got)
				}
			})
		}
	}
}

func TestOpenMPTITVolumePortamentoWithNoteCut(t *testing.T) {
	rows := []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 19, 0xc1, 0}
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			b := testITPitchFixture(rows, 2, true, legacy)
			ref, actual := referencePCMWithChunks(t, b, "2", 160, 128)
			want := rmsWindow(ref, 110, 120) > .001
			got := rmsWindow(actual, 110, 120) > .001
			if got != want {
				t.Errorf("sound after SC1: OpenMPT %v, tracker %v", want, got)
			}
		})
	}
}

func TestITVolumePortamentoMapping(t *testing.T) {
	for i, want := range []uint8{0, 1, 4, 8, 16, 32, 64, 96, 128, 255} {
		arg := uint8(193 + i)
		p, err := unpackITPattern([]byte{0x81, 4, arg, 0, 0x81, 0x40, 0}, 2)
		if err != nil {
			t.Fatal(err)
		}
		for row := range 2 {
			cmd := p.Rows[row][0]
			if !itHasVolumeTonePortamento(cmd) || itVolumePortamentoSpeed(cmd.Volume) != want {
				t.Fatalf("volume %d row %d: %+v", arg, row, cmd)
			}
		}
	}
	for _, arg := range []byte{192, 203, 212, 255} {
		cmd := ModCommand{HasVolume: true, Volume: arg}
		if itHasVolumeTonePortamento(cmd) {
			t.Errorf("volume %d incorrectly classified as tone portamento", arg)
		}
		if arg > 212 {
			if _, err := unpackITPattern([]byte{0x81, 4, arg, 0}, 1); err == nil {
				t.Errorf("unsupported volume %d accepted", arg)
			}
		}
	}
}

func TestITVolumePortamentoVoiceAndMemory(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, compatible := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%v/compatible=%v", legacy, compatible), func(t *testing.T) {
				w := NewTracker(SongAssets{
					IT: true, NumTracks: 2,
					ITCompatibility: ITCompatibility{LinearSlides: true, LegacyModPlug: legacy, CompatibleGxx: compatible},
					Samples:         []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
				}, 48000, 256)
				w.PendingCmds[0] = ModCommand{HasVolume: true, Volume: 198}
				PitchEffectSystem(w)
				if w.TrackMemories[0].LastPortaSpeed != 32 || w.TrackMemories[1].LastPortaSpeed != 0 {
					t.Fatal("portamento memory without voice is incorrect or leaked to another track")
				}
				w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
				VoiceAllocationSystem(w)
				v := w.TrackCursors[0].ActiveVoice
				w.VoiceSamples[v].Position = 123
				w.PendingCmds[0] = ModCommand{Note: 73, HasVolume: true, Volume: 193}
				VoiceAllocationSystem(w)
				PitchEffectSystem(w)
				if w.TrackCursors[0].ActiveVoice != v || w.VoiceSamples[v].Position != 123 {
					t.Fatal("volume-column portamento retriggered the voice")
				}
				base := w.VoicePitches[v].BaseFreqHz
				w.Clock.Tick = 1
				PitchEffectSystem(w)
				if w.VoicePitches[v].BaseFreqHz <= base {
					t.Fatal("zero parameter did not recall portamento speed")
				}
			})
		}
	}
}
