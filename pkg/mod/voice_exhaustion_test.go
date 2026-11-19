package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestITVoiceReclamation(t *testing.T) {
	w := NewTracker(SongAssets{
		IT: true, NumTracks: 1, ITMixGain: 1,
		Samples: []SampleAsset{{ITGlobalVolume: 64}, {ITGlobalVolume: 64, LoopEnable: true}},
	}, 48000, 256)
	w.ITChannelVolume[0], w.ITGlobalVolume = 64, 128
	for i := range MaxVoices {
		w.VoiceLives[i] = VoiceLifecycle{Active: true, ParentTrack: 0, FadeOutVol: 1, ITEnvelopeVolume: 1}
		w.VoiceSamples[i].SampleID = 0
		w.VoiceAmps[i].Volume = 1
	}
	w.TrackCursors[0].ActiveVoice = 0
	w.VoiceAmps[0].Volume = 0
	w.VoiceAmps[7].Volume = .4
	w.VoiceAmps[9].Volume = .5
	w.VoiceSamples[9].SampleID = 1
	if got := w.allocVoice(); got != 9 {
		t.Fatalf("reclaimed voice %d, want quietest looped background voice 9", got)
	}
	if !w.VoiceLives[0].Active || w.TrackCursors[0].ActiveVoice != 0 || w.VoiceLives[9].Active {
		t.Fatal("reclamation disturbed the foreground or left the selected voice active")
	}
	w.VoiceLives[9].Active = true
	w.VoiceAmps[7].Volume = 0
	if got := w.allocVoice(); got != 7 {
		t.Fatalf("reclaimed voice %d, want silent background voice 7", got)
	}
	if got := w.allocVoice(); got != 7 {
		t.Fatalf("free voice %d, want 7", got)
	}
	w.VoiceLives[7].Active = true
	w.VoiceLives[0].ITPastNote = true
	if got := w.allocVoice(); got != 0 || w.TrackCursors[0].ActiveVoice != -1 {
		t.Fatalf("reclaim demoted foreground: voice=%d cursor=%d", got, w.TrackCursors[0].ActiveVoice)
	}
}

func TestOpenMPTVoiceReclamation(t *testing.T) {
	rows := make([]byte, 0, 320*5)
	for range 320 {
		rows = append(rows, 0x81, 3, 60, 2, 0)
	}
	b := mptS70Fixture(t, rows)
	b[50] = 1
	pattern := int(binary.LittleEndian.Uint32(b[210:]))
	binary.LittleEndian.PutUint16(b[pattern+2:], 320)
	instrument := int(binary.LittleEndian.Uint32(b[198:]))
	setTestEnvelope(b[instrument:instrument+554], 304, 3, []int8{64, 0}, []uint16{0, 1})
	b[instrument+306], b[instrument+307] = 1, 1
	ref, actual := mptSharedEffectRenderForDuration(t, b, 6300)
	for _, win := range [][2]int{{100, 200}, {5400, 5500}, {6000, 6100}} {
		want, got := rmsWindow(ref, win[0], win[1]), rmsWindow(actual, win[0], win[1])
		if want < .001 || got < .001 || math.Abs(got/want-1) > .10 {
			t.Errorf("%d-%d ms: OpenMPT RMS %g, tracker RMS %g", win[0], win[1], want, got)
		}
	}
}
