// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func mustWriterFormat(t *testing.T, s string) Format {
	t.Helper()

	f, err := ParseFormat(s)
	if err != nil {
		t.Fatalf("FormatFromString(%q) error: %v", s, err)
	}

	return f
}

func TestNewWriter_InitializesFields(t *testing.T) {
	in := mustWriterFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(in, in)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := new(bytes.Buffer)
	w := NewConvWriter(out, conv)

	if w.out != out {
		t.Fatalf("writer output target not set")
	}

	if w.conv != conv {
		t.Fatalf("writer converter not set")
	}

	const bufferSize = 64 * 1024

	if len(w.buf) != bufferSize {
		t.Fatalf("buffer length = %d, want %d", len(w.buf), bufferSize)
	}
}

func TestWriterWrite_CopyPath(t *testing.T) {
	f := mustWriterFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := new(bytes.Buffer)
	w := NewConvWriter(out, conv)

	in := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	n, err := w.Write(in)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if n != len(in) {
		t.Fatalf("Write returned %d, want %d", n, len(in))
	}

	if !bytes.Equal(out.Bytes(), in) {
		t.Fatalf("written bytes = %v, want %v", out.Bytes(), in)
	}
}

func TestWriterWrite_PartialFrameBuffered_NoOutputYet(t *testing.T) {
	ifmt := mustWriterFormat(t, SLE16Mono44k)
	ofmt := mustWriterFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	conv, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := new(bytes.Buffer)
	w := NewConvWriter(out, conv)

	in := []byte{0x34} // less than one 16-bit frame
	n, err := w.Write(in)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if n != len(in) {
		t.Fatalf("Write returned %d, want %d", n, len(in))
	}

	if out.Len() != 0 {
		t.Fatalf("output length = %d, want 0", out.Len())
	}

	if len(conv.buf) != 1 {
		t.Fatalf("converter buffered bytes = %d, want 1", len(conv.buf))
	}
}

func TestWriterWrite_PropagatesOutputError(t *testing.T) {
	f := mustWriterFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	w := NewConvWriter(errWriter{}, conv)

	_, err = w.Write([]byte{0, 1, 2, 3})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrWrite) {
		t.Fatalf("error %v does not wrap ErrWrite", err)
	}
}

func TestWriterClose_ClearsConverterBuffer(t *testing.T) {
	ifmt := mustWriterFormat(t, SLE16Mono44k)
	ofmt := mustWriterFormat(t, U8Mono8k)
	ofmt.sampleRate = ifmt.sampleRate

	conv, err := NewConverter(ofmt, ifmt)
	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	out := new(bytes.Buffer)
	w := NewConvWriter(out, conv)

	_, err = w.Write([]byte{0x34})
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	if len(conv.buf) != 1 {
		t.Fatalf("converter buffered bytes before Close = %d, want 1", len(conv.buf))
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if len(conv.buf) != 0 {
		t.Fatalf("converter buffered bytes after Close = %d, want 0", len(conv.buf))
	}
}
