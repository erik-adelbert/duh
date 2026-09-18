// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcm

import (
	"sync"
	"time"
)

// Ring is a circular buffer for PCM audio frames. It supports writing arbitrary PCM bytes,
// accumulating them into complete frames, and reading windows of frames without consuming them.
type Ring struct {
	mu sync.Mutex

	Format

	// Always contains complete PCM frames.
	buf []byte

	// Byte positions. Always frame-aligned.
	r int
	w int

	// Number of bytes currently available.
	n int

	// Bytes from the most recent Write that don't yet
	// make up a complete frame.
	pending []byte
}

func NewRing(format Format, dt time.Duration) (r *Ring, err error) {
	fs := format.FrameSize()
	bufsz := format.BufferSize(dt)

	if bufsz < fs {
		err = mkError(ErrRing, "invalid buffer size")
		return
	}

	r = &Ring{
		Format: format,
		buf:    make([]byte, bufsz),

		pending: make([]byte, 0, format.FrameSize()),
	}

	return
}

// Write accepts arbitrary PCM bytes. Bytes are accumulated until complete
// frames can be written to the ring. It never blocks.
//
// Write always consumes the entire input slice and therefore returns
// len(p), nil, unless an error is added in the future.
//
// If the ring is full, the oldest complete frames are discarded.
func (r *Ring) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	n = len(p)

	fs := int(r.FrameSize())

	r.mu.Lock()
	defer r.mu.Unlock()

	// Complete any partial frame left from the previous Write.
	if len(r.pending) > 0 {
		need := min(fs-len(r.pending), len(p))

		r.pending = p[:need]
		p = p[need:]

		if len(r.pending) == fs {
			r.writeFrame(r.pending)
			r.pending = r.pending[:0]
		}
	}

	// Write complete nframe directly from p.
	nframe := len(p) / fs
	for i := range nframe {
		start := i * fs
		r.writeFrame(p[start : start+fs])
	}

	// Save the incomplete trailing frame.
	p = p[nframe*fs:]
	if len(p) > 0 {
		r.pending = p
	}

	return
}

func (r *Ring) writeFrame(frame []byte) {
	fs := int(r.FrameSize())

	if r.n == len(r.buf) {
		r.r += fs
		if r.r == len(r.buf) {
			r.r = 0
		}
		r.n -= fs
	}

	copy(r.buf[r.w:r.w+fs], frame)

	r.w += fs
	if r.w == len(r.buf) {
		r.w = 0
	}

	r.n += fs
}

func (r *Ring) ReadWindow(out []byte, hop int) bool {
	if len(out) == 0 || hop <= 0 {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	fs := int(r.FrameSize())
	nframe := len(out) / fs

	// Only process complete windows.
	if r.n < nframe*fs {
		return false
	}

	nread := nframe * fs

	// Copy window without consuming it.
	first := min(nread, len(r.buf)-r.r)
	copy(out[:first], r.buf[r.r:r.r+first])

	if first < nread {
		copy(out[first:nread], r.buf[:nread-first])
	}

	hop = min(hop, r.n/fs)
	ndiscard := hop * fs

	r.r += ndiscard
	if r.r >= len(r.buf) {
		r.r -= len(r.buf)
	}
	r.n -= ndiscard

	return true
}

func (r *Ring) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.r = 0
	r.w = 0
	r.n = 0
	r.pending = r.pending[:0]
}
