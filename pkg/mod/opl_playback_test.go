package mod

import (
	"math"
	"testing"
)

func testOPLWorld(secondRow ModCommand) *Tracker {
	rows := make([][]ModCommand, 64)
	for i := range rows {
		rows[i] = make([]ModCommand, 1)
	}
	rows[0][0] = ModCommand{Note: 60, Instrument: 1}
	rows[1][0] = secondRow
	w := NewTracker(SongAssets{
		NumTracks: 1, OrderList: []int{0}, Patterns: []PatternAsset{{Rows: rows}},
		Samples: []SampleAsset{{opl: &testOPLPatch, C5Speed: 8363, Volume: 64}},
	}, referenceRate, 5760)
	w.Clock.Speed, w.Clock.Tempo, w.Clock.Tick = 3, 125, -1
	return w
}

func TestOpenMPTOPLTrackerPlayback(t *testing.T) {
	for _, tt := range []struct {
		name string
		row  ModCommand
		ref  []byte
	}{
		{"note", ModCommand{}, nil},
		{"volume", ModCommand{HasVolume: true, Volume: 32}, []byte{0x40, 32, 0}},
		{"cut", ModCommand{Note: 254}, []byte{0x20, 254, 0, 0}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ref := referenceOPL(t, testOPLPatch, tt.ref)
			w := testOPLWorld(tt.row)
			actual := make([]float32, 5760*2)
			if got := w.Read(actual, 5760); got != 5760 {
				t.Fatalf("rendered %d frames", got)
			}
			beforeRef, beforeActual := rmsWindow(ref, 30, 50), rmsWindow(actual, 30, 50)
			if beforeRef < 1e-5 || beforeActual < 1e-5 {
				t.Fatal("silent comparison baseline")
			}
			t.Logf("baseline RMS: OpenMPT %.6f tracker %.6f", beforeRef, beforeActual)
			freq := referenceTickFrequency(t, actual, 1)
			if math.Abs(freq-261.625) > 3 {
				t.Fatalf("pitch %.2f Hz, want about 261.625 Hz", freq)
			}
			if tt.name != "note" {
				want := rmsWindow(ref, 95, 115) / beforeRef
				got := rmsWindow(actual, 95, 115) / beforeActual
				if math.Abs(got-want) > .05 {
					t.Fatalf("relative volume: OpenMPT %.3f, tracker %.3f", want, got)
				}
			}
		})
	}
}

func TestOPLVoiceAllocationAndRelease(t *testing.T) {
	assets := SongAssets{NumTracks: oplChannels + 1, IT: true, ITGlobalVolume: 128, ITMixGain: 1,
		Samples: []SampleAsset{{opl: &testOPLPatch, C5Speed: 8363, Volume: 64, ITGlobalVolume: 64}}}
	for i := range assets.ITChannelVolume {
		assets.ITChannelVolume[i] = 64
	}
	w := NewTracker(assets, 48000, 256)
	for i := range oplChannels + 1 {
		w.PendingCmds[i] = ModCommand{Note: 61, Instrument: 1}
	}
	VoiceAllocationSystem(w)
	for i := range oplChannels {
		if w.TrackCursors[i].ActiveVoice < 0 {
			t.Fatalf("track %d has no voice", i)
		}
		w.PendingCmds[i] = ModCommand{}
	}
	if w.TrackCursors[oplChannels].ActiveVoice != -1 {
		t.Fatal("allocated more than 18 keyed-on voices")
	}
	w.PendingCmds[3] = ModCommand{Note: 255}
	w.PendingCmds[oplChannels] = ModCommand{}
	VoiceAllocationSystem(w)
	old := w.TrackCursors[3].ActiveVoice
	if old < 0 || !w.VoiceLives[old].KeyOff {
		t.Fatal("note-off did not release the OPL voice")
	}
	VoiceKinematicsSystem(w)
	channel := w.VoiceSamples[old].oplChannel - 1
	if w.opl.keyOn[channel]&0x20 != 0 {
		t.Fatal("released voice remained keyed on")
	}
	w.PendingCmds[3] = ModCommand{}
	w.PendingCmds[oplChannels] = ModCommand{Note: 61, Instrument: 1}
	VoiceAllocationSystem(w)
	newVoice := w.TrackCursors[oplChannels].ActiveVoice
	if newVoice < 0 || w.VoiceSamples[newVoice].oplChannel != channel+1 || w.TrackCursors[3].ActiveVoice != -1 {
		t.Fatal("released OPL channel was not reclaimed safely")
	}
	w.PendingCmds[oplChannels] = ModCommand{Note: 254}
	VoiceAllocationSystem(w)
	if w.VoiceLives[newVoice].Active || w.oplVoices[channel] != -1 {
		t.Fatal("cut did not free the OPL channel")
	}
}

func TestOPLPCMVoiceMixAndAllocation(t *testing.T) {
	w := testOPLWorld(ModCommand{})
	w.Assets.NumTracks = 2
	w.Assets.Samples = append(w.Assets.Samples, SampleAsset{
		Data: []float32{0.25, 0.25}, C5Speed: 8363, Volume: 64, LoopEnable: true, LoopEnd: 2,
	})
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	w.PendingCmds[1] = ModCommand{Note: 60, Instrument: 2}
	w.Clock.Tick = 0
	VoiceAllocationSystem(w)
	VoiceKinematicsSystem(w)
	pcm := w.TrackCursors[1].ActiveVoice
	if pcm < 0 || w.TrackCursors[0].ActiveVoice < 0 {
		t.Fatal("PCM and OPL voices were not allocated")
	}
	w.VoiceAmps[pcm].CurrentLeft, w.VoiceAmps[pcm].CurrentRight = .5, .5
	out := make([]float32, 512)
	ResamplerSystem(w, 256)
	MixerAccumulationSystem(w, out, 256)
	var differs bool
	for _, v := range out {
		if math.Abs(float64(v-.125)) > 1e-4 {
			differs = true
		}
	}
	if !differs {
		t.Fatal("OPL did not contribute to PCM mix")
	}
	if got := testing.AllocsPerRun(100, func() {
		ResamplerSystem(w, 256)
		MixerAccumulationSystem(w, out, 256)
	}); got != 0 {
		t.Fatalf("mixed rendering allocated %g times per block", got)
	}
}
