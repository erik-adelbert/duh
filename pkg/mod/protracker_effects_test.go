// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package mod

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"
	"time"
)

func TestProTrackerPitchEffects(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1084:], []byte{1, 0xac, 0x11, 5})
	copy(data[1088:], []byte{0, 0, 0xf, 3})
	copy(data[1100:], []byte{0, 0, 2, 10})
	copy(data[1116:], []byte{1, 0x90, 0x13, 9})
	copy(data[1132:], []byte{0, 0, 3, 0})

	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	var voice VoiceID
	for tick, period := range []int{428, 423, 418, 418, 428, 438, 438, 429, 420, 420, 411, 402} {
		w.Read(buf, 960)
		got := w.TrackCursors[0].ActiveVoice
		if tick == 0 {
			voice = got
		}
		if got < 0 || got != voice {
			t.Fatalf("tick %d unexpectedly changed voice from %d to %d", tick, voice, got)
		}
		if tick == 6 && w.VoiceSamples[got].Position < 32 {
			t.Fatal("tone portamento restarted the looping sample")
		}
		if hz := w.VoicePitches[got].BaseFreqHz; math.Abs(hz-3546895/float64(period)) > 0.001 {
			t.Fatalf("tick %d: period %.2f, want %d", tick, 3546895/hz, period)
		}
	}
}

func TestProTrackerArpeggioReturnsToBasePitch(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x10, 0x47})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick, wantPeriod := range []int{428, 340, 286, 428, 340, 286, 428} {
		w.Read(buf, 960)
		cursor := w.TrackCursors[0]
		if cursor.ActiveVoice < 0 || cursor.modCurrentPeriod != 428 {
			t.Fatalf("tick %d: arpeggio changed the base note", tick)
		}
		if hz := w.VoicePitches[cursor.ActiveVoice].BaseFreqHz; math.Abs(hz-3546895/float64(wantPeriod)) > 0.001 {
			t.Fatalf("tick %d: period %.2f, want %d", tick, 3546895/hz, wantPeriod)
		}
	}
}

func TestProTrackerCombinedPitchAndVolumeEffects(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1084:], []byte{1, 0xac, 0x14, 0x84, 0, 0, 0xf, 3})
	copy(data[1100:], []byte{0, 0, 6, 2})
	copy(data[1116:], []byte{1, 0x90, 3, 5})
	copy(data[1132:], []byte{1, 0x8c, 5, 1})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	periods := []int{428, 428, 433, 428, 435, 433, 428, 423, 418, 418, 413, 408}
	volumes := []int{64, 64, 64, 64, 62, 60, 60, 60, 60, 60, 59, 58}
	for tick, period := range periods {
		w.Read(buf, 960)
		voice := w.TrackCursors[0].ActiveVoice
		if voice < 0 {
			t.Fatalf("tick %d: effect cut the sample", tick)
		}
		hz := w.VoicePitches[voice].BaseFreqHz
		volume := w.VoiceAmps[voice].Volume
		if math.Abs(hz-3546895/float64(period)) > 0.001 || volume != float32(volumes[tick])/64 {
			t.Fatalf("tick %d: period %.2f and volume %g; want %d and %d", tick, 3546895/hz, volume*64, period, volumes[tick])
		}
	}
}

func TestProTrackerNoteCutClearsVolumeWithoutStoppingSample(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x1e, 0xc2})
	copy(data[1118:], []byte{0x0c, 32})
	copy(data[1134:], []byte{0x0e, 0xc0})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick := range 19 {
		w.Read(buf, 960)
		want := float32(0)
		switch {
		case tick < 2:
			want = 1
		case tick >= 12 && tick < 18:
			want = 0.5
		}
		voice := w.TrackCursors[0].ActiveVoice
		if voice < 0 || !w.VoiceLives[voice].Active || w.VoiceAmps[voice].Volume != want {
			t.Fatalf("tick %d: expected the sample to keep playing at volume %g", tick, want)
		}
	}
}

func TestProTrackerRetriggerRestartsFinishedSample(t *testing.T) {
	data := proTrackerFixture(0, 0)
	binary.BigEndian.PutUint16(data[48:], 1) // Disable the fixture's loop.
	copy(data[1086:], []byte{0x1e, 0x93})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick := range 6 {
		w.Read(buf, 960)
		audible := false
		for _, x := range buf {
			audible = audible || math.Abs(float64(x)) > 0.01
		}
		if audible != (tick == 0 || tick == 3) {
			t.Fatalf("tick %d: unexpected audibility %t", tick, audible)
		}
	}
}

func TestProTrackerPatternDelayContinuesEffects(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x1a, 2, 0, 0, 0xf, 3, 0, 0, 0xe, 0xe1})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick, want := range []uint8{64, 62, 60, 58, 56, 54, 54} {
		w.Read(buf, 960)
		if row := w.Clock.Row; row != tick/6 {
			t.Fatalf("tick %d: row %d, want %d", tick, row, tick/6)
		}
		if got := w.TrackCursors[0].modVolume; got != want {
			t.Fatalf("tick %d: volume %d, want %d", tick, got, want)
		}
	}
}

func TestProTrackerPatternLoopWithNoLoop(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1084+61*16+2:], []byte{0x0e, 0x60})
	copy(data[1084+63*16+2:], []byte{0x0e, 0x63})
	dec, err := Decode(bytes.NewReader(data), DecoderOptions{NoLoop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()

	estimator, err := dec.NewDurationEstimator()
	if err != nil {
		t.Fatal(err)
	}
	const ticks = (64 + 3*3) * 6
	wantDuration := ticks * 20 * time.Millisecond
	duration, done, err := estimator.Advance(ticks + 1)
	if err != nil || !done || duration != wantDuration {
		t.Fatalf("duration = %v, done = %v, err = %v; want %v", duration, done, err, wantDuration)
	}
	n, err := io.Copy(io.Discard, dec)
	if err != nil || n != ticks*960*8 {
		t.Fatalf("rendered %d bytes, err = %v; want %d", n, err, ticks*960*8)
	}
}

func TestProTrackerFineSlidesOnDelayedRows(t *testing.T) {
	data := proTrackerFixture(0, 0)
	data[45] = 12
	copy(data[1084:], []byte{
		0, 128, 0x1e, 0x1a,
		1, 0xac, 0x1e, 0xb8,
		0, 0, 0xe, 0xe1,
		0, 0, 0xf, 3,
	})
	copy(data[1100:], []byte{
		3, 0x49, 0x1e, 0x2a,
		0, 0, 0xe, 0xe1,
	})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick := range 6 {
		w.Read(buf, 960)
		period, volume := 118, 4
		if tick >= 3 {
			period, volume = 113, 0
		}
		pitchVoice, volumeVoice := w.TrackCursors[0].ActiveVoice, w.TrackCursors[1].ActiveVoice
		if pitchVoice < 0 || volumeVoice < 0 {
			t.Fatalf("tick %d: missing voice", tick)
		}
		hz := w.VoicePitches[pitchVoice].BaseFreqHz
		gotVolume := w.VoiceAmps[volumeVoice].Volume
		if math.Abs(hz-3546895/float64(period)) > 0.001 || gotVolume != float32(volume)/64 {
			t.Fatalf("tick %d: period %.2f and volume %g; want %d and %d", tick, 3546895/hz, gotVolume*64, period, volume)
		}
	}
	for tick, want := range []int{851, 851, 851, 856, 856, 856, 856} {
		w.Read(buf, 960)
		voice := w.TrackCursors[0].ActiveVoice
		if voice < 0 || math.Abs(w.VoicePitches[voice].BaseFreqHz-3546895/float64(want)) > 0.001 {
			t.Fatalf("tick %d after the fine slide down began: want period %d", tick, want)
		}
	}
}

func TestProTrackerFineVolumeUpOnDelayedRow(t *testing.T) {
	data := proTrackerFixture(0, 0)
	data[45] = 50
	copy(data[1086:], []byte{0x1e, 0xa8, 0, 0, 0xe, 0xe1, 0, 0, 0xf, 3})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	for tick, want := range []float32{58, 58, 58, 64, 64, 64, 64} {
		w.Read(buf, 960)
		voice := w.TrackCursors[0].ActiveVoice
		if voice < 0 || w.VoiceAmps[voice].Volume != want/64 {
			t.Fatalf("tick %d: expected an active voice at volume %g", tick, want)
		}
	}
}

func TestProTrackerPanningPersistsAcrossNotes(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1102:], []byte{0x08, 0x80}) // Center the already playing note.
	copy(data[1116:], []byte{1, 0xac, 0x10, 0})
	copy(data[1134:], []byte{0x08, 0xff})
	copy(data[1150:], []byte{0x08, 0})
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 6*960*2)
	for row, want := range [][2]bool{{true, false}, {true, true}, {true, true}, {false, true}, {true, false}} {
		if n := w.Read(buf, len(buf)/2); n != len(buf)/2 {
			t.Fatalf("row %d: rendered %d frames", row, n)
		}
		var audible [2]bool
		for i, x := range buf[1920:] { // Skip the gain ramp at the row boundary.
			audible[i%2] = audible[i%2] || math.Abs(float64(x)) > 0.01
		}
		if audible != want {
			t.Fatalf("row %d: audible channels %v, want %v", row, audible, want)
		}
		if want[0] && want[1] {
			for i := 1920; i < len(buf); i += 2 {
				if buf[i] != buf[i+1] {
					t.Fatalf("row %d: center panning produced unequal channels", row)
				}
			}
		}
	}
}

func TestProTrackerTremoloPreservesChannelVolume(t *testing.T) {
	data := proTrackerFixture(0, 0)
	data[45] = 20
	copy(data[1086:], []byte{0x17, 0x8f})
	copy(data[1102:], []byte{7, 0})             // Continue with remembered speed and depth.
	copy(data[1132:], []byte{1, 0xac, 0x17, 0}) // A new note resets phase.
	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	want := []int{
		20, 20, 62, 64, 62, 20,
		20, 0, 0, 0, 20, 62,
		20, 20, 20, 20, 20, 20,
		20, 20, 62,
	}
	for tick, volume := range want {
		w.Read(buf, 960)
		cursor := w.TrackCursors[0]
		if cursor.ActiveVoice < 0 || cursor.modVolume != 20 {
			t.Fatalf("tick %d: tremolo changed the stored volume or stopped the voice", tick)
		}
		if got := w.VoiceAmps[cursor.ActiveVoice].Volume; got != float32(volume)/64 {
			t.Fatalf("tick %d: volume %g, want %d", tick, got*64, volume)
		}
	}
}

func TestProTrackerOffsetAndDelay(t *testing.T) {
	data := proTrackerFixture(0, 0)
	data = append(data, make([]byte, 384)...)
	binary.BigEndian.PutUint16(data[42:], 256)
	binary.BigEndian.PutUint16(data[48:], 1) // Disable the fixture's loop.
	copy(data[1084:], []byte{1, 0xac, 0x1e, 0xd2})
	copy(data[1100:], []byte{1, 0xac, 0x19, 1})
	copy(data[1116:], []byte{1, 0xac, 0x09, 0})
	copy(data[1132:], []byte{1, 0xac, 0x09, 2}) // Offset at the end of the sample.

	w := proTrackerEffectTracker(t, data)
	buf := make([]float32, 960*2)
	if w.Read(buf, 960) != 960 || w.Read(buf, 960) != 960 || w.TrackCursors[0].ActiveVoice != -1 {
		t.Fatal("delayed note started before tick 2")
	}
	assertPosition := func(want float64) {
		t.Helper()
		if w.Read(buf[:2], 1) != 1 {
			t.Fatal("no PCM frame")
		}
		v := w.TrackCursors[0].ActiveVoice
		if v < 0 || math.Abs(w.VoiceSamples[v].Position-want-3546895.0/428/48000) > 0.0001 {
			t.Fatalf("voice %d did not start at offset %.0f", v, want)
		}
	}
	assertPosition(0)
	advanceRow := func(remainingTicks int) {
		w.Read(buf, 959)
		for range remainingTicks {
			w.Read(buf, 960)
		}
	}
	advanceRow(3)
	assertPosition(256)
	advanceRow(5)
	assertPosition(256)
	advanceRow(5)
	w.Read(buf[:2], 1)
	if w.Clock.Row != 3 || w.TrackCursors[0].ActiveVoice >= 0 {
		t.Fatalf("out-of-range offset triggered a voice on row %d", w.Clock.Row)
	}
}

func TestProTrackerLEDFilter(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x1e, 0})
	copy(data[1102:], []byte{0x0e, 1})
	w := proTrackerEffectTracker(t, data)
	w.Clock.Tick = 0
	w.PendingCmds[0] = w.Assets.Patterns[0].Rows[0][0]
	w.applyProTrackerFlow()
	stereo := make([]float32, 480*2)
	for i := range len(stereo) / 2 {
		stereo[i*2] = 1 - 2*float32(i%2)
	}
	w.filterProTrackerMix(stereo)
	for i := 240; i < len(stereo); i += 2 {
		if math.Abs(float64(stereo[i])) > 0.01 || stereo[i+1] != 0 {
			t.Fatalf("filter failed to attenuate the left channel without bleed: %v", stereo[i:i+2])
		}
	}
	w.PendingCmds[0] = w.Assets.Patterns[0].Rows[1][0]
	w.applyProTrackerFlow()
	w.filterProTrackerMix(stereo[:2])
	bypass := []float32{1, -1}
	w.filterProTrackerMix(bypass)
	if bypass[0] != 1 || bypass[1] != -1 {
		t.Fatalf("E01 did not bypass the filter: %v", bypass)
	}
}

func proTrackerEffectTracker(t *testing.T, data []byte) *Tracker {
	t.Helper()
	dec, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dec.Close() })
	if warnings := dec.Warnings(); len(warnings) != 0 {
		t.Fatalf("supported effects generated warnings: %v", warnings)
	}
	return dec.tracker
}
