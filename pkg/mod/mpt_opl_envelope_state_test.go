package mod

import "testing"

func TestMPTOPLEndedPitchLifecycle(t *testing.T) {
	for _, tt := range []struct {
		name        string
		noReset     bool
		initialVol  uint8
		retainTicks int
	}{
		{"reset", false, 64, 0},
		{"no-reset", true, 64, 3},
		{"silent", true, 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assets := oplEnvelopePortamentoAssets()
			assets.NumTracks = 2
			assets.ITCompatibility.oplNoResetAtEnvelopeEnd = tt.noReset
			assets.Samples[1].Volume = tt.initialVol
			w := NewTracker(assets, referenceRate, 64)
			w.Clock.Tick = 0
			w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
			VoiceAllocationSystem(w)
			voice := w.TrackCursors[0].ActiveVoice
			VoiceLifecycleSystem(w)
			VoiceKinematicsSystem(w)
			VoiceLifecycleSystem(w)
			if w.VoiceLives[voice].Active || w.TrackCursors[0].ActiveVoice != -1 || w.oplVoices[0] != -1 {
				t.Fatal("envelope end did not release the voice and OPL channel")
			}
			if w.TrackCursors[1].oplHasEndedPitch {
				t.Fatal("retained pitch leaked to another track")
			}
			for i := 0; i < tt.retainTicks; i++ {
				if !w.TrackCursors[0].oplHasEndedPitch {
					t.Fatalf("pitch expired before subsequent tick %d", i+1)
				}
				VoiceLifecycleSystem(w)
			}
			if w.TrackCursors[0].oplHasEndedPitch {
				t.Fatal("pitch persisted after its playback increment expired")
			}
		})
	}
}

func TestMPTOPLEndedPitchClearedByNote(t *testing.T) {
	for _, cmd := range []ModCommand{
		{Note: 254},
		{Note: 73},
		{Note: 73, Instrument: 1, EffectCmd: 3, EffectArg: 16},
	} {
		assets := oplEnvelopePortamentoAssets()
		assets.ITCompatibility.oplNoResetAtEnvelopeEnd = true
		w := NewTracker(assets, referenceRate, 64)
		w.Clock.Tick = 0
		w.PendingCmds[0] = ModCommand{Note: 61, Instrument: 1}
		VoiceAllocationSystem(w)
		VoiceLifecycleSystem(w)
		VoiceKinematicsSystem(w)
		VoiceLifecycleSystem(w)
		if !w.TrackCursors[0].oplHasEndedPitch {
			t.Fatal("no retained pitch to test")
		}
		w.PendingCmds[0] = cmd
		VoiceAllocationSystem(w)
		if w.TrackCursors[0].oplHasEndedPitch {
			t.Fatalf("command %+v retained stale pitch", cmd)
		}
	}
}
