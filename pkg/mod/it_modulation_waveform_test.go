package mod

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestOpenMPTITModulationWaveforms(t *testing.T) {
	for _, family := range []byte{0x40, 0x50} {
		for _, waveform := range []byte{0, 1, 2, 4, 5, 6, 8, 9, 10, 12, 13, 14} {
			for _, legacy := range []bool{false, true} {
				for _, old := range []bool{false, true} {
					t.Run(fmt.Sprintf("S%02X/legacy=%t/old=%t", family|waveform, legacy, old), func(t *testing.T) {
						effect := byte(18)
						if family == 0x50 {
							effect = 25
						}
						rows := []byte{0x81, 15, 60, 1, 32, 19, family | waveform, 0,
							0x81, 8, effect, 0x8f, 0,
							0x81, 15, 60, 1, 32, effect, 0, 0,
							0x81, 8, 19, 0, 0,
							0x81, 8, effect, 0, 0}
						b := testITPitchFixture(rows, 3, true, legacy)
						baseline := testITPitchFixture([]byte{0x81, 7, 60, 1, 32, 0}, 3, true, legacy)
						if old {
							b[44] |= 0x10
							baseline[44] |= 0x10
						}
						ref, actual := referencePCMWithChunks(t, b, "2", 300, 256)
						var refBase, actualBase []float32
						if family == 0x40 {
							refBase, actualBase = referencePCMWithChunks(t, baseline, "2", 300, 256)
						}
						for tick := range 15 {
							ms := tick*20 + 19
							var want, got float64
							if family == 0x40 {
								want = channelRMS(ref, ms, ms+1, 0) / channelRMS(refBase, ms, ms+1, 0)
								got = channelRMS(actual, ms, ms+1, 0) / channelRMS(actualBase, ms, ms+1, 0)
							} else {
								balance := func(pcm []float32) float64 {
									l := channelRMS(pcm, ms, ms+1, 0)
									r := channelRMS(pcm, ms, ms+1, 1)
									return (r - l) / (r + l)
								}
								want, got = balance(ref), balance(actual)
							}
							if math.IsNaN(got) || math.IsNaN(want) || math.Abs(got-want) > .045 {
								t.Errorf("tick %d: OpenMPT %.3f tracker %.3f", tick, want, got)
							}
						}
					})
				}
			}
		}
	}
}

func TestITModulationWaveformCommands(t *testing.T) {
	for family := byte(0x40); family <= 0x50; family += 0x10 {
		for arg := family; arg <= family+15; arg++ {
			p, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0, 0x81, 8, 19, 0, 0}, 2, false)
			if err != nil {
				t.Fatalf("S%02X: %v", arg, err)
			}
			if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err != nil {
				t.Fatalf("MPTM S%02X: %v", arg, err)
			}
			for _, legacy := range []bool{false, true} {
				w := NewTracker(SongAssets{IT: true, NumTracks: 2, ITCompatibility: ITCompatibility{LegacyModPlug: legacy}}, 48000, 256)
				w.PendingCmds[0] = p.Rows[0][0]
				w.TrackMemories[0].itPanbrelloPos = 23
				PitchEffectSystem(w)
				VolumeEffectSystem(w)
				want := arg & 15
				if legacy {
					want &= 7
				} else if want >= 4 {
					want = 0
				}
				field := &w.TrackMemories[0].itTremoloWaveform
				if family == 0x50 {
					field = &w.TrackMemories[0].itPanbrelloWaveform
					if !legacy && w.TrackMemories[0].itPanbrelloPos != 0 || legacy && w.TrackMemories[0].itPanbrelloPos != 23 {
						t.Fatalf("S%02X legacy=%t: wrong panbrello phase", arg, legacy)
					}
				}
				if *field != want || w.TrackMemories[1].itTremoloWaveform != 0 || w.TrackMemories[1].itPanbrelloWaveform != 0 {
					t.Fatalf("S%02X legacy=%t: waveform %d, want %d", arg, legacy, *field, want)
				}
				*field = 15
				w.PendingCmds[0] = p.Rows[1][0]
				PitchEffectSystem(w)
				VolumeEffectSystem(w)
				if *field != want {
					t.Fatalf("S%02X then S00 legacy=%t: waveform %d", arg, legacy, *field)
				}
			}
			if arg&3 == 3 {
				b := testITPitchFixture([]byte{0x81, 8, 19, arg, 0}, 3, true, true)
				if _, err := loadIT(b); err == nil || !strings.Contains(err.Error(), "legacy random") {
					t.Fatalf("S%02X: got %v", arg, err)
				}
			}
		}
	}
}

func TestITRandomPanbrelloSampleAndHold(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1,
		Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x15, EffectArg: 0x53}
	VoiceAllocationSystem(w)
	PitchEffectSystem(w)
	VolumeEffectSystem(w)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x18, EffectArg: 0x4f}
	seen := map[int8]bool{}
	var first int8
	for tick := range 32 {
		w.Clock.Tick = tick
		VoiceKinematicsSystem(w)
		value := w.TrackMemories[0].itPanbrelloOffset
		if value < -64 || value > 63 {
			t.Fatalf("random panbrello out of range: %d", value)
		}
		if tick%4 == 0 {
			first = value
			seen[value] = true
		} else if value != first {
			t.Fatalf("tick %d: sample-and-hold changed from %d to %d", tick, first, value)
		}
	}
	if len(seen) < 3 {
		t.Fatal("random panbrello did not change across hold periods")
	}
}
