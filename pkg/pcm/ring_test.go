// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package pcm

import (
	"bytes"
	"testing"
)

func TestRingReadFramesWrapsReadIndex(t *testing.T) {
	format, err := ParseFormat(SLE16Mono8k)
	if err != nil {
		t.Fatalf("FormatFromString: %v", err)
	}

	dt := format.Duration(4 * int64(format.FrameSize()))

	ring, err := NewRing(format, dt)
	if err != nil {
		t.Fatalf("NewRing: %v", err)
	}

	data := bytes.Repeat([]byte{0x11, 0x22}, 4*int(format.FrameSize()))
	_, _ = ring.Write(data)

	firstReadBytes := 3 * int(format.FrameSize())

	first := make([]byte, firstReadBytes)

	if n := ring.ReadFrames(first); n != firstReadBytes {
		t.Fatalf("ReadFrames first pass = %d, want %d", n, firstReadBytes)
	}

	if got, want := ring.r, 3*format.FrameSize(); got != int(want) {
		t.Fatalf("ring.r after first read = %d, want %d (frame size = %d)", got, want, format.FrameSize())
	}

	secondReadBytes := int(format.FrameSize())

	second := make([]byte, secondReadBytes)

	if n := ring.ReadFrames(second); n != secondReadBytes {
		t.Fatalf("ReadFrames second pass = %d, want %d", n, secondReadBytes)
	}

	if got, want := ring.r, 0; got != want {
		t.Fatalf("ring.r after wrapped read = %d, want %d (frame size = %d)", got, want, format.FrameSize())
	}
}

func TestRingReadFramesWrapsPastEndOfBuffer(t *testing.T) {
	format, err := ParseFormat(SLE16Mono8k)
	if err != nil {
		t.Fatalf("FormatFromString: %v", err)
	}

	dt := format.Duration(4 * int64(format.FrameSize()))

	ring, err := NewRing(format, dt)
	if err != nil {
		t.Fatalf("NewRing: %v", err)
	}
	// Seed the ring so the read head is near the end and the read crosses the wrap boundary.
	ring.buf = make([]byte, 8)
	copy(ring.buf[0:4], []byte{0x10, 0x20, 0x30, 0x40})
	copy(ring.buf[4:8], []byte{0x50, 0x60, 0x70, 0x80})
	ring.r = 6
	ring.w = 2
	ring.n = 4

	out := make([]byte, 4)
	if n := ring.ReadFrames(out); n != 4 {
		t.Fatalf("ReadFrames cross-wrap = %d, want 4", n)
	}

	if got, want := ring.r, 2; got != want {
		t.Fatalf("ring.r after cross-wrap read = %d, want %d", got, want)
	}
}
