// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcm

import (
	"sync"
	"time"
)

type Ring struct {
	mu sync.Mutex

	format Format

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
	bufsz := format.BufferSize(dt)

	if bufsz <= 0 {
		err = mkError(ErrRing, "invalid buffer size")
		return
	}

	r = &Ring{
		format: format,
		buf:    make([]byte, bufsz),
	}

	return
}

// Write adds arbitrary bytes from the audio backend.
//
// It never blocks waiting for the consumer.
// If the ring is full, the oldest complete PCM frames are dropped.
func (r *Ring) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	fs := int(r.format.FrameSize())

	// Complete any partial frame left from the previous Write.
	if len(r.pending) > 0 {
		need := min(fs-len(r.pending), len(p))

		r.pending = p[:need]
		p = p[need:]

		if len(r.pending) == fs {

			r.mu.Lock()
			r.writeFrame(r.pending)
			r.mu.Unlock()

			r.pending = r.pending[:0]
		}
	}

	// Write complete nframe directly from p.
	r.mu.Lock()
	nframe := len(p) / fs
	for i := range nframe {
		start := i * fs
		r.writeFrame(p[start : start+fs])
	}
	r.mu.Unlock()

	// Save the incomplete trailing frame.
	p = p[nframe*fs:]
	if len(p) > 0 {
		r.pending = p
	}

	return nframe * fs, nil
}

// writeFrame must be called with r.mu held.
func (r *Ring) writeFrame(frame []byte) {
	fs := int(r.format.FrameSize())

	// Full: discard the oldest frame.
	if r.n == len(r.buf) {
		r.r += fs
		r.r %= len(r.buf)

		r.n -= fs
	}

	copy(r.buf[r.w:r.w+fs], frame)

	r.w += fs
	r.w %= len(r.buf)

	r.n += fs
}

// ReadFrames reads up complete PCM frames.
//
// It never returns a partial frame.
// It returns the number of bytes read.
func (r *Ring) ReadFrames(out []byte) int {
	if len(out) == 0 {
		return 0
	}

	fs := int(r.format.FrameSize())
	nframe := len(out) / fs

	nframe = min(nframe, r.n/fs)
	nread := nframe * fs

	r.mu.Lock()
	defer r.mu.Unlock()

	// First contiguous portion.
	first := min(nread, len(r.buf)-r.r)
	copy(out[:first], r.buf[r.r:r.r+first])

	// Wrapped portion.
	if first < nread {
		copy(out[first:nread], r.buf[:nread-first])
	}

	r.r += nread
	r.r %= len(r.buf)

	r.n -= nread

	return nread
}

func (r *Ring) AvailableFrames() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.n / int(r.format.FrameSize())
}

func (r *Ring) CapacityFrames() int {
	return len(r.buf) / int(r.format.FrameSize())
}

func (r *Ring) Format() Format {
	return r.format
}
