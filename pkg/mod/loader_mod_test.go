// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package mod

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
)

func proTrackerFixture(track int, tune byte) []byte {
	const sampleSize = 128
	data := make([]byte, 1084+1024+sampleSize)
	data[950], data[951] = 1, 0x7f
	copy(data[1080:], "M.K.")
	binary.BigEndian.PutUint16(data[42:], sampleSize/2)
	data[44], data[45] = tune, 64
	binary.BigEndian.PutUint16(data[46:], 16)              // Loop from byte 32.
	binary.BigEndian.PutUint16(data[48:], 32)              // Two 32-sample cycles.
	copy(data[1084+track*4:], []byte{0x01, 0xac, 0x10, 0}) // Sample 1, period 428.
	for i := range sampleSize {
		data[1084+1024+i] = byte(int8(math.Round(100 * math.Sin(2*math.Pi*float64(i)/32))))
	}
	return data
}

func TestProTrackerPlayback(t *testing.T) {
	testCases := []struct {
		name        string
		track, side int
		tune        byte
		period      float64
	}{
		{"first left", 0, 0, 0, 428},
		{"first right", 1, 1, 7, 407},
		{"second right", 2, 1, 8, 453},
		{"second left", 3, 0, 0, 428},
	}
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			dec, err := Decode(bytes.NewReader(proTrackerFixture(tt.track, tt.tune)), DecoderOptions{NoLoop: true})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = dec.Close() })
			pcm, err := io.ReadAll(dec)
			if err != nil {
				t.Fatal(err)
			}
			const frameSize = 8
			if got, want := len(pcm)/frameSize, 64*6*960; got != want {
				t.Fatalf("rendered %d frames, want %d", got, want)
			}
			read := func(frame, side int) float32 {
				return math.Float32frombits(binary.LittleEndian.Uint32(pcm[frame*frameSize+side*4:]))
			}
			// Measure after the short sample would have ended without looping.
			const start, end = 6000, 54000
			var crossings int
			prev := read(start-1, tt.side)
			for i := start; i < end; i++ {
				if other := read(i, 1-tt.side); other != 0 {
					t.Fatalf("frame %d leaked into opposite channel: %g", i, other)
				}
				x := read(i, tt.side)
				if prev <= 0 && x > 0 {
					crossings++
				}
				prev = x
			}
			want := 3546895 / tt.period / 32
			if math.Abs(float64(crossings)-want) > 1 {
				t.Errorf("measured %d Hz after looping, want %.2f Hz", crossings, want)
			}
		})
	}
}

func TestProTrackerStoredPatternOutsideSong(t *testing.T) {
	active := proTrackerFixture(0, 0)
	sampleOffset := modHeaderSize + modPatternSize
	data := make([]byte, len(active)+modPatternSize)
	copy(data, active[:sampleOffset])
	data[953] = 1             // Inactive order, but its pattern is stored.
	data[sampleOffset] = 0xff // Not a playable cell; must not be interpreted.
	copy(data[sampleOffset+modPatternSize:], active[sampleOffset:])

	dec, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	assets := dec.tracker.Assets
	if !reflect.DeepEqual(assets.OrderList, []int{0}) || len(assets.Patterns) != 1 {
		t.Fatalf("inactive pattern affected song: orders %v, patterns %d", assets.OrderList, len(assets.Patterns))
	}
	for i, b := range active[sampleOffset:] {
		if got, want := assets.Samples[0].Data[i], float32(int8(b))/128; got != want {
			t.Fatalf("sample byte %d decoded as %g, want %g", i, got, want)
		}
	}

	data[953] = 0
	if _, err := Decode(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "file size does not match") {
		t.Fatalf("unaccounted extra bytes: got %v, want file size error", err)
	}

	active[953] = 1
	withoutExtraPattern, err := Decode(bytes.NewReader(active))
	if err != nil {
		t.Fatalf("inactive order with no stored pattern: %v", err)
	}
	defer withoutExtraPattern.Close()
}

func TestProTrackerDiagnostics(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x1e, 0x36})
	copy(data[1090:], []byte{0x0e, 0x33})
	copy(data[1094:], []byte{0x0e, 0x42})
	dec, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = dec.Close() })
	want := []string{"ProTracker MOD: E3x effect is not supported", "ProTracker MOD: E4x effect is not supported"}
	if got := dec.Warnings(); !reflect.DeepEqual(got, want) {
		t.Errorf("warnings = %q, want %q", got, want)
	}
	testProTrackerMalformed(t, data)
}

func TestProTrackerFlowAndDuration(t *testing.T) {
	active := proTrackerFixture(0, 0)
	sampleOffset := modHeaderSize + modPatternSize
	data := make([]byte, len(active)+modPatternSize)
	copy(data, active[:sampleOffset])
	copy(data[sampleOffset+modPatternSize:], active[sampleOffset:])
	data[950], data[953] = 2, 1
	copy(data[1086:], []byte{0x1f, 3, 0, 0, 0xf, 150, 0, 0, 0xd, 0x12})
	target := sampleOffset + 12*modChannels*4
	copy(data[target+2:], []byte{0xf, 0, 0, 0, 0xd, 0})

	dec, err := Decode(bytes.NewReader(data), DecoderOptions{NoLoop: true})
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	if warnings := dec.Warnings(); len(warnings) != 0 {
		t.Fatalf("supported effects generated warnings: %v", warnings)
	}
	estimator, err := dec.NewDurationEstimator()
	if err != nil {
		t.Fatal(err)
	}
	duration, done, err := estimator.Advance(7)
	const wantFrames = 4800 // Two three-tick rows at 150 BPM.
	if err != nil || !done || duration.Milliseconds() != 100 {
		t.Fatalf("estimated duration = %s, done %t, err %v; want 100ms", duration, done, err)
	}
	pcm, err := io.ReadAll(dec)
	if err != nil || len(pcm) != wantFrames*8 {
		t.Fatalf("rendered %d frames, err %v; want %d", len(pcm)/8, err, wantFrames)
	}
	if c := dec.tracker.Clock; c.Order != 2 || c.Row != 0 {
		t.Fatalf("final pattern break left order %d, row %d", c.Order, c.Row)
	}
}

func TestProTrackerPositionJumpAndBreak(t *testing.T) {
	for _, tt := range []struct {
		name    string
		effects []byte
		row     int
	}{
		{"jump before break", []byte{0xb, 2, 0, 0, 0xd, 0x12}, 12},
		{"break before jump", []byte{0xd, 0x12, 0, 0, 0xb, 2}, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			active := proTrackerFixture(0, 0)
			sampleOffset := modHeaderSize + modPatternSize
			data := make([]byte, len(active)+2*modPatternSize)
			copy(data, active[:sampleOffset])
			copy(data[sampleOffset+2*modPatternSize:], active[sampleOffset:])
			data[950], data[953], data[954] = 3, 1, 2
			copy(data[modHeaderSize+2:], tt.effects)
			lastRow := modHeaderSize + 2*modPatternSize + tt.row*modChannels*4
			copy(data[lastRow+2:], []byte{0xb, 0})

			dec, err := Decode(bytes.NewReader(data), DecoderOptions{NoLoop: true})
			if err != nil {
				t.Fatal(err)
			}
			defer dec.Close()
			estimator, err := dec.NewDurationEstimator()
			if err != nil {
				t.Fatal(err)
			}
			duration, done, err := estimator.Advance(13)
			if err != nil || !done || duration.Milliseconds() != 240 {
				t.Fatalf("duration = %v, done = %v, err = %v; want 240ms", duration, done, err)
			}
			if n, err := io.Copy(io.Discard, dec); err != nil || n != 2*6*960*8 {
				t.Fatalf("NoLoop rendered %d bytes, err = %v; want two rows", n, err)
			}

			w := proTrackerEffectTracker(t, data)
			if n := w.Read(make([]float32, (6*960+1)*2), 6*960+1); n != 6*960+1 || w.Clock.Order != 2 || w.Clock.Row != tt.row {
				t.Fatalf("position after jump = order %d row %d; want order 2 row %d", w.Clock.Order, w.Clock.Row, tt.row)
			}
			if n := w.Read(make([]float32, 6*960*2), 6*960); n != 6*960 || w.Clock.Order != 0 || w.Clock.Row != 0 || w.Clock.SongFinished {
				t.Fatalf("playback without NoLoop did not follow the jump back to order 0")
			}
		})
	}
}

func TestProTrackerVolumeAcrossNotes(t *testing.T) {
	data := proTrackerFixture(0, 0)
	copy(data[1086:], []byte{0x1c, 20, 0, 0, 0xf, 3})
	copy(data[1102:], []byte{0xa, 0x02})
	copy(data[1116:], []byte{0x01, 0xac, 0, 0}) // Note without a new sample number.
	copy(data[1134:], []byte{0x1c, 80})         // Set volume above 64 clamps.
	dec, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	w := dec.tracker
	buf := make([]float32, 960*2)
	for tick, want := range []uint8{20, 20, 20, 20, 18, 16, 16, 16, 16, 64} {
		if got := w.Read(buf, 960); got != 960 {
			t.Fatalf("tick %d: rendered %d frames", tick, got)
		}
		voice := w.TrackCursors[0].ActiveVoice
		if voice < 0 || w.VoiceAmps[voice].Volume != float32(want)/64 {
			t.Fatalf("tick %d: got voice %d, channel volume %d; want volume %d", tick, voice, w.TrackCursors[0].modVolume, want)
		}
	}
}

func testProTrackerMalformed(t *testing.T, data []byte) {
	t.Helper()
	badLoop := bytes.Clone(data)
	badLoop[48] = 1
	for _, tt := range []struct {
		name, diagnostic string
		data             []byte
		options          DecoderOptions
	}{
		{"pattern", "truncated pattern", data[:1084], DecoderOptions{}},
		{"sample", "truncated sample 1", data[:len(data)-1], DecoderOptions{}},
		{"loop", "sample 1 loop exceeds sample length", badLoop, DecoderOptions{}},
		{"sequence", "supports only sequence 0", data, DecoderOptions{Sequence: 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(bytes.NewReader(tt.data), tt.options)
			if err == nil || !strings.Contains(err.Error(), tt.diagnostic) {
				t.Fatalf("Decode error = %v, want %q", err, tt.diagnostic)
			}
		})
	}
}
