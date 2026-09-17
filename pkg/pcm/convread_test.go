// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 <Your Name>
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

func TestConvReader_CopyPath(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)
	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}

	output, err := io.ReadAll(OpenConv(bytes.NewReader(input), conv))

	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, input) {
		t.Fatalf("output = %v, want %v", output, input)
	}
}

func TestConvReader_FlushesPartialFrameAtEOF(t *testing.T) {
	ifmt := mustReaderFormat(t, SLE16Mono44k)
	ofmt := mustReaderFormat(t, U8Mono8k)

	ofmt.sampleRate = ifmt.sampleRate

	conv, err := NewConverter(ofmt, ifmt)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	output, err := io.ReadAll(OpenConv(bytes.NewReader([]byte{0x34}), conv))

	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, []byte{0x80}) {
		// 0x34 -> 0x34 0x00 -> 0x80 (unsigned 8-bit)
		t.Fatalf("output = %v, want [0x80]", output)
	}
}

func TestConvReader_SeekResetsPendingOutput(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}

	reader := OpenConv(bytes.NewReader(input), conv)

	buffer := make([]byte, 2)

	if _, err := reader.Read(buffer); err != nil {
		t.Fatalf("Read error: %v", err)
	}

	offset, err := reader.Seek(4, io.SeekStart)

	if err != nil {
		t.Fatalf("Seek error: %v", err)
	}

	if offset != 4 {
		t.Fatalf("Seek returned %d, want 4", offset)
	}

	output, err := io.ReadAll(reader)

	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}

	if !bytes.Equal(output, input[4:]) {
		t.Fatalf("output = %v, want %v", output, input[4:])
	}
}

func TestReader_ReadAtDoesNotAffectSequentialRead(t *testing.T) {
	f := mustReaderFormat(t, SLE16Stereo44k)

	conv, err := NewConverter(f, f)

	if err != nil {
		t.Fatalf("NewConverter error: %v", err)
	}

	input := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	reader := OpenConv(bytes.NewReader(input), conv)
	output := make([]byte, 4)

	n, err := reader.ReadAt(output, 4)
	if err != nil {
		t.Fatalf("ReadAt error: %v", err)
	}
	if n != len(output) || !bytes.Equal(output, input[4:]) {
		t.Fatalf("ReadAt = %v, want %v", output, input[4:])
	}

	n, err = io.ReadFull(reader, output)
	if err != nil {
		t.Fatalf("sequential Read error: %v", err)
	}
	if n != len(output) || !bytes.Equal(output, input[:4]) {
		t.Fatalf("sequential output = %v, want %v", output, input[:4])
	}
}

func mustReaderFormat(t *testing.T, text string) Format {
	t.Helper()

	format, err := ParseFormat(text)

	if err != nil {
		t.Fatalf("FormatFromString(%q) error: %v", text, err)
	}

	return format
}
