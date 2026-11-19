package mod

import "testing"

func TestOPLInstrumentOnlyMemoryAndCut(t *testing.T) {
	_, assets := transitionFixture(false, false, false)
	assets.ITInstruments = make([]ITInstrument, 2)
	for i := range assets.ITInstruments {
		assets.ITInstruments[i].GlobalVolume = 128
		for n := range assets.ITInstruments[i].Samples {
			assets.ITInstruments[i].Samples[n] = uint8(i + 1)
			assets.ITInstruments[i].Notes[n] = uint8(n)
		}
	}
	assets.ITInstruments[0].Notes[60] = 72
	assets.ITInstruments[1].Notes[60] = 65
	w := NewTracker(assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Instrument: 2}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("instrument without previous note started a voice")
	}
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	if voice < 0 || w.VoicePitches[voice].Note != 73 {
		t.Fatal("initial note mapping was not applied")
	}
	w.PendingCmds[0] = ModCommand{Note: 254}
	VoiceAllocationSystem(w)
	w.PendingCmds[0] = ModCommand{Instrument: 2}
	VoiceAllocationSystem(w)
	voice = w.TrackCursors[0].ActiveVoice
	if voice < 0 || w.VoicePitches[voice].Note != 66 || w.VoiceSamples[voice].oplChannel == 0 {
		t.Fatal("switch after cut did not reuse the original pattern note")
	}
	w.PendingCmds[0] = ModCommand{Note: 254}
	VoiceAllocationSystem(w)
	w.Assets.ITInstruments[0].Samples[60] = 0
	w.PendingCmds[0] = ModCommand{Instrument: 1}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("empty instrument mapping started a voice")
	}
}

func TestOPLInstrumentOnlySwitchProfileIsGated(t *testing.T) {
	_, assets := transitionFixture(false, false, false)
	assets.ITCompatibility.oplInstrumentSwitch = false
	w := NewTracker(assets, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Instrument: 2}
	VoiceAllocationSystem(w)
	if w.TrackCursors[0].ActiveVoice != voice || w.VoiceSamples[voice].SampleID != 0 {
		t.Fatal("instrument-only switch changed playback without the compatibility profile")
	}
}
