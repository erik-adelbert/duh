// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wav

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

func testFormat(t *testing.T, text string) Format {
	t.Helper()
	f, err := ParseFormat(text)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func testWAV(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	enc, err := Encode(&out, testFormat(t, "s16c1r44100"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enc.Write([]byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := enc.Close(); err != nil {
		t.Fatal(err)
	}
	data := out.Bytes()
	binary.LittleEndian.PutUint32(data[40:], 4)
	return data
}

type readFailureSeeker struct {
	*bytes.Reader
	readErr    error
	restoreErr error
	restorePos int64
}

func (r *readFailureSeeker) Read(p []byte) (int, error) {
	if r.readErr != nil {
		return 0, r.readErr
	}
	return r.Reader.Read(p)
}

func (r *readFailureSeeker) Seek(off int64, whence int) (int64, error) {
	if r.restoreErr != nil && whence == io.SeekStart && off == r.restorePos {
		return 0, r.restoreErr
	}
	return r.Reader.Seek(off, whence)
}

func TestDecoderReadAtErrors(t *testing.T) {
	source := &readFailureSeeker{Reader: bytes.NewReader(testWAV(t))}
	dec, err := Decode(source)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := source.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal(err)
	}
	source.restorePos = pos

	buf := make([]byte, 3)
	if n, err := dec.ReadAt(buf, 3); n != 1 || err != io.EOF || buf[0] != 4 {
		t.Fatalf("short ReadAt = (%d, %v, %v), want (1, EOF, [4 ...])", n, err, buf)
	}
	if actual, _ := source.Seek(0, io.SeekCurrent); actual != pos {
		t.Fatalf("position = %d, want %d", actual, pos)
	}

	source.readErr = errors.New("source read failed")
	if n, err := dec.ReadAt(buf, 0); n != 0 || !errors.Is(err, ErrDecode) || !strings.Contains(err.Error(), source.readErr.Error()) {
		t.Fatalf("read failure = (%d, %v)", n, err)
	}
	source.restoreErr = errors.New("restore failed")
	if _, err := dec.ReadAt(buf, 2); err == nil || !strings.Contains(err.Error(), source.readErr.Error()) || !strings.Contains(err.Error(), source.restoreErr.Error()) {
		t.Fatalf("read and restore failures = %v", err)
	}
	source.readErr = nil
	if _, err := source.Reader.Seek(pos, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if _, err := dec.ReadAt(buf[:2], 2); err == nil || !strings.Contains(err.Error(), source.restoreErr.Error()) {
		t.Fatalf("restore failure = %v", err)
	}
}

func TestDecoderSeek(t *testing.T) {
	data := testWAV(t)
	nonseekable := struct{ io.Reader }{bytes.NewReader(data)}
	dec, err := Decode(nonseekable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dec.Seek(0, io.SeekStart); !errors.Is(err, ErrESPIPE) {
		t.Fatalf("non-seekable Seek error = %v", err)
	}

	dec, err = Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	first, err := io.ReadAll(dec)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dec.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	second, err := io.ReadAll(dec)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("read after seek = %v, %v; want %v", second, err, first)
	}
}

type partialWriter struct {
	bytes.Buffer
	call    int
	partial int
	failure error
}

func (w *partialWriter) Write(p []byte) (int, error) {
	w.call++
	if w.call == w.partial {
		n, _ := w.Buffer.Write(p[:len(p)/2])
		return n, w.failure
	}
	return w.Buffer.Write(p)
}

func TestEncoderPartialWrites(t *testing.T) {
	failure := errors.New("write failed")
	for _, tc := range []struct {
		name    string
		partial int
		err     error
	}{
		{"header-short", 1, nil},
		{"header-error", 1, failure},
		{"data-short", 2, nil},
		{"data-error", 2, failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := &partialWriter{partial: tc.partial, failure: tc.err}
			enc, err := Encode(out, testFormat(t, "s16c1r44100"))
			if err != nil {
				t.Fatal(err)
			}
			data := []byte{1, 2, 3, 4, 5, 6}
			n, err := enc.Write(data)
			if tc.partial == 1 {
				if n != 0 || err == nil {
					t.Fatalf("header write = (%d, %v)", n, err)
				}
				n, err = enc.Write(data)
			}
			if n != len(data) || err != nil {
				t.Fatalf("data write = (%d, %v)", n, err)
			}
			err = enc.Flush()
			if tc.partial == 2 {
				if err == nil || (tc.err == nil && !errors.Is(err, io.ErrShortWrite)) || (tc.err != nil && !errors.Is(err, tc.err)) {
					t.Fatalf("partial Flush error = %v", err)
				}
				err = enc.Flush()
			}
			if err != nil {
				t.Fatal(err)
			}
			got := out.Bytes()
			if len(got) != headersz+len(data) || !bytes.Equal(got[headersz:], data) || string(got[:4]) != "RIFF" {
				t.Fatalf("output = %v, want one header followed by %v", got, data)
			}
		})
	}
}

func TestEncoderConcurrentHeaders(t *testing.T) {
	formats := []Format{testFormat(t, "s16c1r44100"), testFormat(t, "u8c2r48000")}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f := formats[i%len(formats)]
			var out bytes.Buffer
			enc, err := Encode(&out, f)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := enc.Write([]byte{0}); err != nil {
				t.Error(err)
				return
			}
			header := out.Bytes()
			if got := binary.LittleEndian.Uint32(header[offspr:]); got != uint32(f.SampleRate()) {
				t.Errorf("header rate = %d, want %d", got, f.SampleRate())
			}
			if got := binary.LittleEndian.Uint16(header[offnch:]); got != uint16(f.ChannelCount()) {
				t.Errorf("header channels = %d, want %d", got, f.ChannelCount())
			}
			if got := binary.LittleEndian.Uint16(header[offbit:]); got != uint16(f.BitDepth()) {
				t.Errorf("header bits = %d, want %d", got, f.BitDepth())
			}
		}()
	}
	wg.Wait()
}
