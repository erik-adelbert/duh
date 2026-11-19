package mod

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestOpenMPTITGlissando(t *testing.T) {
	tests := []struct {
		name string
		rows []byte
	}{
		{"up Gxx", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"down Gxx", []byte{0x81, 11, 72, 1, 19, 0x11, 0, 0x81, 9, 60, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"Lxx", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 12, 0, 0}},
		{"volume portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 5, 72, 198, 0, 0x81, 4, 198, 0}},
		{"S10 disables", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0x10, 0, 0x81, 8, 7, 0, 0}},
		{"S00 recalls", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0, 0, 0x81, 8, 7, 0, 0}},
		{"S1F enables", []byte{0x81, 11, 60, 1, 19, 0x1f, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"same row volume portamento", []byte{0x81, 3, 60, 1, 0, 0x81, 13, 72, 198, 19, 0x11, 0, 0x81, 4, 198, 0}},
		{"pitch slide after portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 6, 8, 0}},
		{"new note resets portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 3, 61, 1, 0}},
		{"finetune before glissando", []byte{0x81, 11, 60, 1, 19, 0x2a, 0, 0x81, 8, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}},
		{"finetune after portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 19, 0x25, 0, 0x81, 8, 7, 0, 0}},
		{"vibrato and volume portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 13, 72, 198, 8, 0x85, 0, 0x81, 12, 198, 8, 0x85, 0}},
		{"disable with volume portamento", []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 5, 72, 198, 0, 0x81, 12, 198, 19, 0x10, 0}},
	}
	for _, tt := range tests {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/legacy=%t/linear=%t", tt.name, legacy, linear), func(t *testing.T) {
					b := testITPitchFixture(tt.rows, 3, linear, legacy)
					ref, actual := referencePCMWithChunks(t, b, "2", 240, 256)
					refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
					for tick := range 12 {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.Abs(got-want) > .003 {
							t.Errorf("tick %d: OpenMPT ratio %.5f, tracker %.5f", tick, want, got)
						}
					}
				})
			}
		}
	}
}

func TestITGlissandoState(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 2,
		ITCompatibility: ITCompatibility{LinearSlides: true},
		Samples:         []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1, EffectCmd: 0x15, EffectArg: 0x11}
	VoiceAllocationSystem(w)
	VolumeEffectSystem(w)
	if !w.TrackMemories[0].itGlissando || w.TrackMemories[1].itGlissando {
		t.Fatal("glissando was not enabled only on the selected track")
	}

	w.PendingCmds[0] = ModCommand{Note: 73, EffectCmd: 0x03, EffectArg: 8}
	VoiceAllocationSystem(w)
	PitchEffectSystem(w)
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	base := w.VoicePitches[voice].BaseFreqHz
	VoiceKinematicsSystem(w)
	if base <= noteToHz(60) || w.VoicePitches[voice].StepIncrement <= base/48000 ||
		w.VoicePitches[voice].BaseFreqHz != base {
		t.Fatal("glissando did not quantize output while preserving slide state")
	}
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x15, EffectArg: 0x10}
	VolumeEffectSystem(w)
	if w.TrackMemories[0].itGlissando || w.TrackMemories[0].ITLastSpecial != 0x10 {
		t.Fatal("S10 did not disable glissando and update Sxx memory")
	}
	w.PendingCmds[0].EffectArg = 0x11
	VolumeEffectSystem(w)
	w.PendingCmds[0].EffectArg = 0
	VolumeEffectSystem(w)
	if !w.TrackMemories[0].itGlissando || w.TrackMemories[0].ITLastSpecial != 0x11 {
		t.Fatal("S00 did not recall S11")
	}
}

func TestOpenMPTITGlissandoSampleRates(t *testing.T) {
	rows := []byte{0x81, 11, 60, 1, 19, 0x11, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}
	for _, rate := range []uint32{8363, 11025, 22050} {
		for _, legacy := range []bool{false, true} {
			for _, linear := range []bool{false, true} {
				t.Run(fmt.Sprintf("rate=%d/legacy=%t/linear=%t", rate, legacy, linear), func(t *testing.T) {
					b := testITPitchFixture(rows, 3, linear, legacy)
					binary.LittleEndian.PutUint32(b[256+60:], rate)
					ref, actual := referencePCMWithChunks(t, b, "2", 240, 256)
					refBase, actualBase := referenceTickFrequency(t, ref, 0), referenceTickFrequency(t, actual, 0)
					for tick := 3; tick < 12; tick++ {
						want := referenceTickFrequency(t, ref, tick) / refBase
						got := referenceTickFrequency(t, actual, tick) / actualBase
						if math.Abs(got-want) > .003 {
							t.Errorf("tick %d: OpenMPT ratio %.5f, tracker %.5f", tick, want, got)
						}

					}

				})
			}
		}
	}
}

func TestITGlissandoFineTuneBaseline(t *testing.T) {
	rows := []byte{0x81, 11, 60, 1, 19, 0x2a, 0, 0x81, 8, 19, 0x10, 0, 0x81, 9, 72, 7, 8, 0, 0x81, 8, 7, 0, 0}
	for _, legacy := range []bool{false, true} {
		for _, linear := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy=%t/linear=%t", legacy, linear), func(t *testing.T) {
				ref, actual := referencePCMWithChunks(t, testITPitchFixture(rows, 3, linear, legacy), "2", 240, 256)
				for tick := range 12 {
					want, got := referenceTickFrequency(t, ref, tick), referenceTickFrequency(t, actual, tick)
					if math.Abs(got/want-1) > .003 {
						t.Errorf("tick %d: OpenMPT %.3f, tracker %.3f", tick, want, got)
					}
				}
			})
		}
	}
}

func TestITGlissandoCommandsAndMemory(t *testing.T) {
	for arg := byte(0x10); arg <= 0x1f; arg++ {
		p, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0, 0x81, 8, 19, 0, 0}, 2, false)
		if err != nil {
			t.Fatalf("S%02X: %v", arg, err)
		}
		if _, err := unpackITPatternMode([]byte{0x81, 8, 19, arg, 0}, 1, true); err != nil {
			t.Fatalf("MPTM S%02X: %v", arg, err)
		}
		w := NewTracker(SongAssets{IT: true, NumTracks: 1}, 48000, 256)
		w.PendingCmds[0] = p.Rows[0][0]
		VolumeEffectSystem(w)
		if got := w.TrackMemories[0].itGlissando; got != (arg != 0x10) {
			t.Fatalf("S%02X: glissando=%t", arg, got)
		}
		w.TrackMemories[0].itGlissando = arg == 0x10
		w.PendingCmds[0] = p.Rows[1][0]
		VolumeEffectSystem(w)
		if got := w.TrackMemories[0].itGlissando; got != (arg != 0x10) {
			t.Fatalf("S%02X then S00: glissando=%t", arg, got)
		}
	}
}
