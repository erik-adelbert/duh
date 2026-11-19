package mod

import (
	"fmt"
	"testing"
)

func TestOPLPCMTransitionInstrumentActions(t *testing.T) {
	for _, firstOPL := range []bool{false, true} {
		for _, porta := range []bool{false, true} {
			for _, continuePast := range []bool{false, true} {
				t.Run(fmt.Sprintf("opl=%v/porta=%v/continue=%v", firstOPL, porta, continuePast), func(t *testing.T) {
					_, assets := transitionFixture(firstOPL, porta, false)
					assets.ITInstruments = make([]ITInstrument, 2)
					for i := range assets.ITInstruments {
						instrument := &assets.ITInstruments[i]
						instrument.GlobalVolume = 128
						if continuePast {
							instrument.NewNoteAction = 1
						}
						for note := range instrument.Samples {
							instrument.Samples[note], instrument.Notes[note] = uint8(i+1), uint8(note)
						}
					}
					first, second := uint8(1), uint8(2)
					if firstOPL {
						first, second = second, first
					}
					w := NewTracker(assets, 48000, 256)
					w.PendingCmds[0] = ModCommand{Note: 61, Instrument: first}
					VoiceAllocationSystem(w)
					VoiceKinematicsSystem(w)
					old := w.TrackCursors[0].ActiveVoice
					oldChannel := w.VoiceSamples[old].oplChannel
					w.PendingCmds[0] = ModCommand{Note: 73, Instrument: second}
					if porta {
						w.PendingCmds[0].EffectCmd, w.PendingCmds[0].EffectArg = 3, 16
					}
					VoiceAllocationSystem(w)
					current := w.TrackCursors[0].ActiveVoice
					if current < 0 || w.VoiceSamples[current].SampleID != int(second)-1 {
						t.Fatal("transition did not select the new sample")
					}
					if continuePast && !porta {
						if current == old || !w.VoiceLives[old].Active || !w.VoiceLives[old].ITPastNote {
							t.Fatal("normal note failed to preserve the past voice")
						}
					} else if current != old {
						t.Fatal("cut or portamento transition did not reuse stopped voice")
					}
					if oldChannel != 0 && (porta || !continuePast) && w.oplVoices[oldChannel-1] != -1 {
						t.Fatal("stopped OPL voice retained its channel")
					}
					VoiceKinematicsSystem(w)
					w.PendingCmds[0] = ModCommand{Note: 254}
					VoiceAllocationSystem(w)
					if w.TrackCursors[0].ActiveVoice != -1 || w.VoiceLives[current].Active {
						t.Fatal("cut did not stop the new voice")
					}
					if continuePast && !porta && !w.VoiceLives[old].Active {
						t.Fatal("cut stopped the past voice")
					}
				})
			}
		}
	}
}

func TestOPLPortamentoPatchSwap(t *testing.T) {
	_, assets := transitionFixture(true, true, false)
	assets.ITCompatibility.oplPortaSampleRetrigger = false
	other := testOPLPatch
	other[3] = 15
	assets.Samples = append(assets.Samples, SampleAsset{opl: &other, C5Speed: 8363, Volume: 32, ITGlobalVolume: 64})
	w := NewTracker(assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 2}
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	channel := w.VoiceSamples[voice].oplChannel
	oldPitch := w.VoicePitches[voice].BaseFreqHz
	w.PendingCmds[0] = ModCommand{Note: 73, Instrument: 3, EffectCmd: 3, EffectArg: 16}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != voice || w.VoiceSamples[voice].oplChannel != channel ||
		w.VoiceSamples[voice].SampleID != 2 || w.opl.patches[channel-1] != other ||
		w.VoicePitches[voice].BaseFreqHz != oldPitch || w.VoiceAmps[voice].Volume != .5 {
		t.Fatal("OPL patch swap did not preserve the active voice and pitch")
	}
	VoiceKinematicsSystem(w)
	if w.opl.keyOn[channel-1]&0x20 == 0 {
		t.Fatal("OPL patch swap unexpectedly keyed off")
	}
}
