package mod

import "testing"

func TestOPLDuplicateSampleCutReusesChannel(t *testing.T) {
	instrument := ITInstrument{NewNoteAction: 1, DuplicateCheck: 2, GlobalVolume: 128}
	for i := range instrument.Samples {
		instrument.Samples[i], instrument.Notes[i] = 1, uint8(i)
	}
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITInstruments: []ITInstrument{instrument},
		Samples: []SampleAsset{{opl: &testOPLPatch, C5Speed: 8363, Volume: 64}},
	}, 48000, 256)
	w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	channel := w.VoiceSamples[voice].oplChannel
	w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
	VoiceAllocationSystem(w)
	if got := w.TrackCursors[0].ActiveVoice; got != voice || w.VoiceSamples[got].oplChannel != channel ||
		w.oplVoices[channel-1] != got {
		t.Fatal("duplicate cut did not free and reuse the OPL voice and channel")
	}
	VoiceKinematicsSystem(w)
	if w.opl.keyOn[channel-1]&0x20 == 0 {
		t.Fatal("replacement OPL note was not keyed on")
	}
}

func TestOPLInstrumentNewNoteActions(t *testing.T) {
	for _, tt := range []struct {
		name       string
		action     uint8
		pastActive bool
		pastKeyOff bool
		pastFading bool
	}{
		{"cut", 0, false, false, false},
		{"continue", 1, true, false, false},
		{"off", 2, true, true, true},
		{"fade", 3, true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			instrument := ITInstrument{NewNoteAction: tt.action, Fadeout: 32, GlobalVolume: 128}
			for i := range instrument.Samples {
				instrument.Samples[i], instrument.Notes[i] = 1, uint8(i)
			}
			assets := SongAssets{IT: true, NumTracks: 1, ITGlobalVolume: 128, ITMixGain: 1,
				ITInstruments: []ITInstrument{instrument}, ITChannelVolume: [MaxTracks]uint8{64},
				Samples: []SampleAsset{{opl: &testOPLPatch, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64}}}
			w := NewTracker(assets, 48000, 256)
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			VoiceKinematicsSystem(w)
			past := w.TrackCursors[0].ActiveVoice
			pastChannel := w.VoiceSamples[past].oplChannel
			w.PendingCmds[0] = ModCommand{Note: 62, Instrument: 1}
			VoiceAllocationSystem(w)
			current := w.TrackCursors[0].ActiveVoice
			if current < 0 {
				t.Fatal("new note not allocated")
			}
			if !tt.pastActive {
				if current != past || w.VoiceSamples[current].oplChannel != pastChannel {
					t.Fatal("cut voice channel was not reused")
				}
				return
			}
			if current == past || !w.VoiceLives[past].Active || !w.VoiceLives[past].ITPastNote ||
				w.VoiceLives[past].KeyOff != tt.pastKeyOff || w.VoiceLives[past].ITFade != tt.pastFading {
				t.Fatalf("past voice state: %+v", w.VoiceLives[past])
			}
			VoiceKinematicsSystem(w)
			if keyedOn := w.opl.keyOn[pastChannel-1]&0x20 != 0; keyedOn == tt.pastKeyOff {
				t.Fatalf("past voice keyed on = %v", keyedOn)
			}
			w.stopVoice(past)
			if w.TrackCursors[0].ActiveVoice != current || !w.VoiceLives[current].Active {
				t.Fatal("stopping past voice affected current note")
			}
		})
	}
}

func TestOPLReleasedSampleVoiceIsNotArtificiallyFaded(t *testing.T) {
	w := testOPLWorld(ModCommand{})
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	VoiceAllocationSystem(w)
	voice := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{Note: 255}
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	for range 30 {
		VoiceLifecycleSystem(w)
	}
	if !w.VoiceLives[voice].Active || w.VoiceLives[voice].FadeOutVol != 1 {
		t.Fatal("released OPL voice was artificially faded")
	}
}

func TestOPLPitchPanAndRetrigger(t *testing.T) {
	w := testOPLWorld(ModCommand{})
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	channel := w.VoiceSamples[v].oplChannel - 1
	original := w.opl.keyOn[channel]
	w.VoicePitches[v].BaseFreqHz *= 2
	w.VoiceAmps[v].Panning = -1
	VoiceKinematicsSystem(w)
	if w.opl.keyOn[channel] == original || w.opl.keyOn[channel]&0x20 == 0 {
		t.Fatal("pitch update did not reach OPL")
	}
	out := make([]float32, 512)
	MixerAccumulationSystem(w, out, 256)
	for i := 20; i < len(out); i += 2 {
		if out[i+1] != 0 {
			t.Fatalf("hard-left panning produced right sample %g", out[i+1])
		}
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x13, EffectArg: 1}
	w.TrackMemories[0].RetrigCount = 1
	PitchEffectSystem(w)
	if w.opl.keyOn[channel]&0x20 != 0 {
		t.Fatal("retrigger did not key off")
	}
	VoiceKinematicsSystem(w)
	if w.opl.keyOn[channel]&0x20 == 0 {
		t.Fatal("retrigger did not key on")
	}
}
