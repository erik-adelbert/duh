// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func TestOtoPrepareReader(t *testing.T) {
	inputFormat, err := pcm.NewFormat(pcm.SignedIntBE, 16, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}

	outputFormat, err := pcm.NewFormat(pcm.Float, 32, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("converts input to output", func(t *testing.T) {
		backend := OtoBackend{Format: inputFormat, ofmt: outputFormat}
		source := bytes.NewReader([]byte{0x40, 0x00, 0x40, 0x00})
		reader, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if seeker == nil || reader != seeker {
			t.Fatal("seekable input should produce a seekable converted reader")
		}

		var output [4]byte
		if _, err := io.ReadFull(reader, output[:]); err != nil {
			t.Fatal(err)
		}
		if got := math.Float32frombits(binary.LittleEndian.Uint32(output[:])); got != 0.5 {
			t.Fatalf("converted sample = %v, want 0.5", got)
		}
	})

	t.Run("unchanged format", func(t *testing.T) {
		backend := OtoBackend{Format: outputFormat, ofmt: outputFormat}
		source := bytes.NewReader([]byte{1, 2, 3, 4})
		reader, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if reader != source || seeker != source {
			t.Fatal("matching formats should preserve the original reader")
		}
	})

	t.Run("non-seekable input", func(t *testing.T) {
		backend := OtoBackend{Format: inputFormat, ofmt: outputFormat}
		source := struct{ io.Reader }{bytes.NewReader([]byte{0x40, 0x00})}
		_, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if seeker != nil {
			t.Fatal("non-seekable input should not produce a seeker")
		}
	})
}

func TestOtoPlayerDurationUsesOutputFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"converted depth", "S16c1r44100", "f32c1r44100"},
		{"converted channels", "s16c4r44100", "f32c2r44100"},
		{"unchanged", "s16c2r44100", "s16c2r44100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifmt, err := pcm.ParseFormat(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			ofmt, err := pcm.ParseFormat(tt.out)
			if err != nil {
				t.Fatal(err)
			}
			backend := OtoBackend{Format: ifmt, ofmt: ofmt}
			source := bytes.NewReader(make([]byte, int(ifmt.BufferSize(time.Second))))
			start, duration, seekable, err := inspectReader(source, ifmt)
			if err != nil {
				t.Fatal(err)
			}
			reader, seeker, err := backend.prepReader(source)
			if err != nil {
				t.Fatal(err)
			}
			if !seekable {
				t.Fatal("source should be seekable")
			}
			tracker := &tracker{reader: reader, seeker: seeker}
			tracker.offset.Store(remap(start, ifmt.FrameSize(), ofmt.FrameSize()))
			player := OtoPlayer{Format: ifmt, ofmt: ofmt, tracker: tracker, duration: duration}
			if got := player.Duration(); got != time.Second {
				t.Fatalf("duration = %v, want %v", got, time.Second)
			}
			if got := player.Format; got != ifmt {
				t.Fatalf("player format = %v, want input format %v", got, ifmt)
			}
		})
	}
}

type fakePlayback struct {
	reader   *tracker
	buffered int
	seeks    int
}

func (p *fakePlayback) Seek(offset int64, whence int) (int64, error) {
	p.seeks++
	p.buffered = 0
	return p.reader.Seek(offset, whence)
}

func (p *fakePlayback) BufferedSize() int { return p.buffered }

type seekCountingReader struct {
	*bytes.Reader
	seeks int
}

func (r *seekCountingReader) Seek(offset int64, whence int) (int64, error) {
	r.seeks++
	return r.Reader.Seek(offset, whence)
}

func TestOtoPlayerPositionAndSeek(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		out  string
	}{
		{"converted", "S16c1r44100", "f32c1r44100"},
		{"unchanged", "s16c1r44100", "s16c1r44100"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ifmt, err := pcm.ParseFormat(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			ofmt, err := pcm.ParseFormat(tt.out)
			if err != nil {
				t.Fatal(err)
			}
			source := &seekCountingReader{Reader: bytes.NewReader(make([]byte, int(ifmt.BufferSize(time.Second))))}
			backend := OtoBackend{Format: ifmt, ofmt: ofmt}
			start, duration, seekable, err := inspectReader(source, ifmt)
			if err != nil || !seekable {
				t.Fatalf("inspectAudioReader = (%v, %v)", seekable, err)
			}
			reader, seeker, err := backend.prepReader(source)
			if err != nil {
				t.Fatal(err)
			}
			tracker := &tracker{reader: reader, seeker: seeker}
			tracker.offset.Store(remap(start, ifmt.FrameSize(), ofmt.FrameSize()))
			playback := &fakePlayback{reader: tracker}
			player := &OtoPlayer{Format: ifmt, ofmt: ofmt, tracker: tracker, playback: playback, duration: duration}
			outFrame := int(ofmt.FrameSize())
			if _, err := io.ReadFull(tracker, make([]byte, 12*outFrame)); err != nil {
				t.Fatal(err)
			}
			playback.buffered = 4 * outFrame
			before := source.seeks
			for range 3 {
				if got, want := player.Position(), 8*ifmt.FrameSize(); got != want {
					t.Fatalf("Position = %d, want %d", got, want)
				}
				if got := player.Duration(); got != time.Second {
					t.Fatalf("Duration = %v, want 1s", got)
				}
			}
			if source.seeks != before || playback.seeks != 0 {
				t.Fatal("position or duration sought the source")
			}
			if _, err := io.ReadFull(tracker, make([]byte, 2*outFrame)); err != nil {
				t.Fatal(err)
			}
			playback.buffered += 2 * outFrame
			if got, err := player.Seek(2*ifmt.FrameSize(), io.SeekCurrent); err != nil || got != 10*ifmt.FrameSize() {
				t.Fatalf("relative seek = (%d, %v)", got, err)
			}
			if playback.seeks != 1 || player.Position() != 10*ifmt.FrameSize() {
				t.Fatalf("position after seek = %d; seeks = %d", player.Position(), playback.seeks)
			}
			if got, err := player.Seek(-2*ifmt.FrameSize(), io.SeekEnd); err != nil || got != int64(ifmt.BufferSize(time.Second))-2*ifmt.FrameSize() {
				t.Fatalf("end seek = (%d, %v)", got, err)
			}
		})
	}
}

func TestPreparePositionReaderInitialOffsetAndErrors(t *testing.T) {
	f, err := pcm.ParseFormat("s16c1r44100")
	if err != nil {
		t.Fatal(err)
	}
	source := bytes.NewReader(make([]byte, int(f.BufferSize(time.Second))))
	if _, err := source.Seek(100, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	start, duration, seekable, err := inspectReader(source, f)
	if err != nil {
		t.Fatal(err)
	}
	if start != 100 || duration != time.Second || !seekable {
		t.Fatalf("initial offset = %d, duration = %v, seekable = %v", start, duration, seekable)
	}
	if pos, err := source.Seek(0, io.SeekCurrent); err != nil || pos != 100 {
		t.Fatalf("restored position = %d, %v", pos, err)
	}
	failure := errors.New("seek failed")
	_, _, _, err = inspectReader(&failedPositionReader{err: failure}, f)
	if !errors.Is(err, failure) {
		t.Fatalf("prepare error = %v, want %v", err, failure)
	}
	pipe, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pipe.Close()   //nolint:errcheck // can't do much if this fails
	defer writer.Close() //nolint:errcheck // can't do much if this fails
	_, duration, seekable, err = inspectReader(pipe, f)
	if err != nil || seekable || duration != 0 {
		t.Fatalf("inspect pipe = (duration %v, seekable %v, error %v)", duration, seekable, err)
	}
}

type failedPositionReader struct{ err error }

func (r *failedPositionReader) Read([]byte) (int, error)       { return 0, r.err }
func (r *failedPositionReader) Seek(int64, int) (int64, error) { return 0, r.err }

func TestOtoPlayerPositionNonSeekable(t *testing.T) {
	f, err := pcm.ParseFormat("s16c1r44100")
	if err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewBuffer([]byte{1, 2, 3, 4})
	_, duration, seekable, err := inspectReader(reader, f)
	if err != nil || seekable {
		t.Fatalf("inspect non-seekable reader = (%v, %v)", seekable, err)
	}
	tracker := &tracker{reader: reader}

	playback := &fakePlayback{reader: tracker}
	player := &OtoPlayer{Format: f, ofmt: f, tracker: tracker, playback: playback, duration: duration}
	if _, err := io.ReadFull(tracker, make([]byte, 4)); err != nil {
		t.Fatal(err)
	}
	if got := player.Position(); got != 4 {
		t.Fatalf("Position = %d, want 4", got)
	}
	if _, err := player.Seek(0, io.SeekStart); !errors.Is(err, ErrESPIPE) {
		t.Fatalf("Seek error = %v, want ErrESPIPE", err)
	}
}

func TestOtoPlayerPositionConcurrentRead(t *testing.T) {
	f, err := pcm.ParseFormat("s16c1r44100")
	if err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewBuffer(make([]byte, 8192))
	_, duration, _, err := inspectReader(reader, f)
	if err != nil {
		t.Fatal(err)
	}
	tracker := &tracker{reader: reader}
	player := &OtoPlayer{
		Format: f, ofmt: f, tracker: tracker,
		playback: &fakePlayback{reader: tracker}, duration: duration,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := io.Copy(io.Discard, tracker); err != nil {
			t.Error(err)
		}
	}()
	for range 1000 {
		_ = player.Position()
	}
	wg.Wait()
	if got := player.Position(); got != 8192 {
		t.Fatalf("Position = %d, want 8192", got)
	}
}
