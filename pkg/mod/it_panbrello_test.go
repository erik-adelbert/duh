package mod

import (
	"fmt"
	"math"
	"math/rand"
	"testing"
)

func TestOpenMPTITPanbrello(t *testing.T) {
	fastWrap := []byte{0x81, 11, 60, 1, 25, 0xff, 0}
	for range 24 {
		fastWrap = append(fastWrap, 0x81, 8, 25, 0, 0)
	}
	tests := []struct {
		name string
		rows []byte
	}{
		{"Y4F", []byte{0x81, 11, 60, 1, 25, 0x4f, 0}},
		{"fast wrap", fastWrap},
		{"partial memory", []byte{0x81, 11, 60, 1, 25, 0x4f, 0, 0x81, 8, 25, 0x02, 0, 0x81, 8, 25, 0x80, 0, 0x81, 8, 25, 0, 0}},
		{"hold and resume", []byte{0x81, 11, 60, 1, 25, 0x4f, 0, 0, 0x81, 8, 25, 0, 0}},
		{"note reset", []byte{0x81, 11, 60, 1, 25, 0x4f, 0, 0x81, 3, 60, 1, 0, 0x81, 8, 25, 0, 0}},
		{"pan reset", []byte{0x81, 11, 60, 1, 25, 0x4f, 0, 0x81, 8, 24, 64, 0, 0x81, 8, 25, 0, 0}},
		{"before note", []byte{0x81, 8, 25, 0x4f, 0, 0x81, 11, 60, 1, 25, 0, 0}},
		{"volume pan", []byte{0x81, 15, 60, 1, 144, 25, 0x4f, 0}},
		{"surround", []byte{0x81, 11, 60, 1, 19, 0x91, 0, 0x81, 8, 25, 0x4f, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, speed := range []byte{1, 6} {
				t.Run(fmt.Sprintf("%s/legacy=%v/speed=%d", tt.name, legacy, speed), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, speed, true, legacy)
					ref, actual := referencePCMWithChunks(t, b, "2", 480, 960)
					first := 0
					if tt.name == "before note" {
						first = int(speed)
					}
					for tick := first; tick < 24; tick++ {
						if tt.name == "pan reset" && tick == int(speed) {
							// OpenMPT uses a faster ramp for explicit panning changes.
							continue
						}
						balance := func(pcm []float32) float64 {
							left := channelRMS(pcm, tick*20+15, tick*20+19, 0)
							right := channelRMS(pcm, tick*20+15, tick*20+19, 1)
							if left+right < 1e-6 {
								t.Fatalf("tick %d: silent comparison", tick)
							}
							return (right - left) / (right + left)
						}
						want, got := balance(ref), balance(actual)
						if math.Abs(got-want) > .035 {
							t.Errorf("tick %d: OpenMPT balance %.3f, tracker %.3f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITPanbrelloRandomState(t *testing.T) {
	for _, hold := range []bool{false, true} {
		for _, speed := range []uint8{0, 1, 4, 15} {
			t.Run(fmt.Sprintf("hold=%t/speed=%d", hold, speed), func(t *testing.T) {
				profile := itPanbrelloProfile{preciseWaveform: true, hold: hold, sampleAndHold: true}
				w := NewTracker(SongAssets{
					IT: true, NumTracks: 1, ITCompatibility: ITCompatibility{itPanbrelloProfile: &profile},
					Samples: []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
				}, 48000, 256)
				w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
				VoiceAllocationSystem(w)
				voice := w.TrackCursors[0].ActiveVoice
				mem := &w.TrackMemories[0]
				mem.itPanbrelloWaveform = 3
				w.PendingCmds[0] = ModCommand{EffectCmd: 0x18, EffectArg: speed<<4 | 15}
				random := rand.New(rand.NewSource(1))
				var pos uint8
				var held int8
				for tick := range 40 {
					w.Clock.Tick = tick
					candidate := int8(random.Intn(128) - 64)
					if pos == 0 || pos >= speed {
						pos = 0
						held = candidate
					}
					pos++
					w.updateITPanbrello()
					if mem.itPanbrelloPos != pos || mem.itPanbrelloRandom != held {
						t.Fatalf("tick %d: position=%d held=%d, want %d and %d",
							tick, mem.itPanbrelloPos, mem.itPanbrelloRandom, pos, held)
					}
					wantOffset := float32((int(held)*15+2)/8) / 128
					if got := w.VoiceAmps[voice].itPanbrelloOffset; got != wantOffset {
						t.Fatalf("tick %d: offset=%v, want %v", tick, got, wantOffset)
					}
					if hold && mem.itPanbrelloOffset != held || !hold && mem.itPanbrelloOffset != 0 {
						t.Fatalf("tick %d: persistent offset=%d", tick, mem.itPanbrelloOffset)
					}
				}
				if got, want := w.itRandom.Intn(128), random.Intn(128); got != want {
					t.Fatalf("random stream advanced differently: got %d, want %d", got, want)
				}
			})
		}
	}
}

func TestITPanbrelloMemoryAndPastVoice(t *testing.T) {
	inst := ITInstrument{NewNoteAction: 1, GlobalVolume: 128}
	for i := range inst.Samples {
		inst.Samples[i], inst.Notes[i] = 1, uint8(i)
	}
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2, ITInstruments: []ITInstrument{inst},
		Samples: []SampleAsset{{Data: []float32{1, 1}, Volume: 64, C5Speed: 8363}},
	}, 48000, 4)
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x18, EffectArg: 0x4f}
	VoiceKinematicsSystem(w)
	if w.TrackMemories[0].itPanbrello != 0x4f || w.TrackMemories[0].itPanbrelloPos != 0 ||
		w.TrackMemories[1].itPanbrello != 0 {
		t.Fatal("panbrello memory without a voice or track isolation is incorrect")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x18}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	for range 8 {
		VoiceKinematicsSystem(w)
	}
	if w.VoiceAmps[voice].itPanbrelloOffset == 0 {
		t.Fatal("panbrello did not change foreground panning")
	}
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	current := w.TrackCursors[0].ActiveVoice
	if current == voice || !w.VoiceLives[voice].ITPastNote || w.VoiceAmps[voice].itPanbrelloOffset != 0 ||
		w.TrackMemories[0].itPanbrelloOffset != 0 {
		t.Fatal("past note or new note retained panbrello offset")
	}
	VoiceKinematicsSystem(w)
	if w.VoiceAmps[voice].itPanbrelloOffset != 0 || w.VoiceAmps[current].itPanbrelloOffset != 0 {
		t.Fatal("panbrello leaked to another voice")
	}
}

func TestITPanbrelloPackedReuseAndUnsupportedWaveforms(t *testing.T) {
	pattern, err := unpackITPattern([]byte{
		0x81, 8, 25, 0x4f, 0,
		0x81, 0x80, 0,
		0x81, 8, 25, 0, 0,
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	for row, want := range []uint8{0x4f, 0x4f, 0} {
		cmd := pattern.Rows[row][0]
		if cmd.EffectCmd != 0x18 || cmd.EffectArg != want {
			t.Fatalf("row %d: effect %02x%02x, want Y%02x", row, cmd.EffectCmd, cmd.EffectArg, want)
		}
	}
	for _, arg := range []byte{0x30, 0x31, 0x32, 0x33} {
		if _, _, err := mapITEffect(19, arg); err == nil {
			t.Fatalf("unsupported S%02x waveform control was accepted", arg)
		}
	}
}
