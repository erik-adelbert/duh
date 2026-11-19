package mod

import (
	"encoding/binary"
	"math"
	"testing"
)

func testWorld() *Tracker {
	return NewTracker(SongAssets{
		NumTracks: 1,
		Samples:   []SampleAsset{{Data: make([]float32, 1024), C5Speed: 8363, Volume: 48}},
	}, 48000, 64)
}

func TestS3MPatternVolumeAndEffect(t *testing.T) {
	data := make([]byte, 2)
	data = append(data, 0x20, 0x40, 1, 0xc0, 20, 15, 2, 0)
	data = append(data, make([]byte, 63)...)
	binary.LittleEndian.PutUint16(data, uint16(len(data)-2))
	pattern, err := unpackS3MPattern(append([]byte{0}, data...), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	cmd := pattern.Rows[0][0]
	if cmd.Note != 60 || cmd.Instrument != 1 || !cmd.HasVolume || cmd.Volume != 20 || cmd.EffectCmd != 0x12 || cmd.EffectArg != 2 {
		t.Fatalf("decoded command = %+v", cmd)
	}
}

func TestNoteDelayVolumeAndOffset(t *testing.T) {
	w := testWorld()
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1, Volume: 20, HasVolume: true, EffectCmd: 0x15, EffectArg: 0xd3}
	for tick := 0; tick <= 3; tick++ {
		w.Clock.Tick = tick
		VoiceAllocationSystem(w)
		if got := w.TrackCursors[0].ActiveVoice; (got != -1) != (tick == 3) {
			t.Fatalf("tick %d: voice = %d", tick, got)
		}
	}
	v := w.TrackCursors[0].ActiveVoice
	if got := w.VoiceAmps[v].Volume; got != 20.0/64 {
		t.Fatalf("volume = %v", got)
	}
	w.Clock.Tick = 0
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1, EffectCmd: 0x12, EffectArg: 2}
	VoiceAllocationSystem(w)
	v = w.TrackCursors[0].ActiveVoice
	if got := w.VoiceSamples[v].Position; got != 512 {
		t.Fatalf("sample offset = %v", got)
	}
	w.PendingCmds[0].EffectArg = 0
	VoiceAllocationSystem(w)
	v = w.TrackCursors[0].ActiveVoice
	if got := w.VoiceSamples[v].Position; got != 512 {
		t.Fatalf("remembered sample offset = %v", got)
	}
}

func TestS3MVolumeSlides(t *testing.T) {
	for _, tt := range []struct {
		arg  uint8
		tick int
		want float32
	}{
		{0xf2, 0, 30}, {0xf2, 1, 32}, {0x2f, 0, 34},
		{0x03, 0, 32}, {0x03, 1, 29}, {0x20, 1, 34},
	} {
		w := testWorld()
		w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
		VoiceAllocationSystem(w)
		v := w.TrackCursors[0].ActiveVoice
		w.VoiceAmps[v].Volume = 0.5
		w.PendingCmds[0] = ModCommand{EffectCmd: 0x0a, EffectArg: tt.arg}
		w.Clock.Tick = tt.tick
		VolumeEffectSystem(w)
		if got := w.VoiceAmps[v].Volume * 64; got != tt.want {
			t.Errorf("D%02X tick %d = %v, want %v", tt.arg, tt.tick, got, tt.want)
		}
	}
}

func TestArpeggioAndRetrigger(t *testing.T) {
	w := testWorld()
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x10, EffectArg: 0x7c}
	w.Clock.Tick = 1
	PitchEffectSystem(w)
	ratio := (w.VoicePitches[v].BaseFreqHz + w.VoicePitches[v].VibratoOffset) / w.VoicePitches[v].BaseFreqHz
	if math.Abs(ratio-math.Pow(2, 7.0/12)) > 1e-10 {
		t.Fatalf("arpeggio frequency ratio = %v", ratio)
	}
	w.PendingCmds[0] = ModCommand{EffectCmd: 0x13, EffectArg: 0xf2}
	for tick := 0; tick <= 2; tick++ {
		w.Clock.Tick = tick
		w.VoiceSamples[v].Position = 100
		w.VoiceSamples[v].Reverse = true
		PitchEffectSystem(w)
	}
	if w.VoiceSamples[v].Position != 0 || w.VoiceSamples[v].Reverse || w.VoiceAmps[v].Volume != 1 {
		t.Fatalf("retrigger position = %v, reverse = %v, volume = %v", w.VoiceSamples[v].Position, w.VoiceSamples[v].Reverse, w.VoiceAmps[v].Volume)
	}
}

func TestOneShotTailMixed(t *testing.T) {
	w := testWorld()
	w.Assets.Samples[0].Data = []float32{1, 1}
	w.PendingCmds[0] = ModCommand{Note: 60, Instrument: 1}
	VoiceAllocationSystem(w)
	v := w.TrackCursors[0].ActiveVoice
	w.VoicePitches[v].StepIncrement = 1
	w.VoiceAmps[v].CurrentLeft, w.VoiceAmps[v].TargetLeft = 1, 1
	ResamplerSystem(w, 4)
	out := make([]float32, 8)
	MixerAccumulationSystem(w, out, 4)
	if out[0] != 1 || out[2] != 1 || out[4] != 0 || w.VoiceLives[v].Active {
		t.Fatalf("one-shot output = %v, active = %v", out, w.VoiceLives[v].Active)
	}
}
