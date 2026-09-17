// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audio

import (
	"iter"
	"slices"
	"sync"

	"github.com/erik-adelbert/duh/internal/xiter"
)

type S284 = []Fp284

type Buffer struct {
	S284
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(Buffer)
	},
}

// GetBuffer retrieves a Buffer from the pool.
func GetBuffer(size int) *Buffer {
	buf := bufferPool.Get().(*Buffer)

	if size > cap(buf.S284) {
		buf.S284 = slices.Grow(buf.S284, size-cap(buf.S284))
	}

	buf.S284 = buf.S284[:size]

	return buf
}

// PutBuffer returns a Buffer to the pool.
func PutBuffer(buf *Buffer) {
	buf.Reset()
	bufferPool.Put(buf)
}

func NewBuffer(args ...int) (*Buffer, func()) {
	var size, capacity int

	switch len(args) {
	case 0:
		size, capacity = 0, 0
	case 1:
		size = args[0]
		capacity = args[0]
	case 2:
		size = args[0]
		capacity = args[1]
	}

	buf := GetBuffer(max(size, capacity))

	buf.Resize(size)

	closer := func() {
		PutBuffer(buf)
	}

	return buf, closer
}

func (b *Buffer) Len() int {
	return len(b.S284)
}

func (b *Buffer) Cap() int {
	return cap(b.S284)
}

func (b *Buffer) Resize(n int) {
	b.S284 = b.S284[:n]
}

func (b *Buffer) Slice(i, j int) *Buffer {
	return &Buffer{
		S284: b.S284[i:j],
	}
}

func (b *Buffer) Clear() {
	clear(b.S284)
}

func (b *Buffer) Reset() {
	b.S284 = b.S284[:0]
}

const (
	DecayShift = 8
	DecayMask  = (1 << DecayShift) - 1
)

func (b *Buffer) Decay(offL, offR *Fp284, sampleCount int) {
	l0, r0 := *offL, *offR

	if l0 == 0 && r0 == 0 {
		return
	}

	for i := range b.S284[:sampleCount] {
		// Apply simple exponential decay to left and right offsets
		l := (l0 + (l0 + ((-l0)>>31)&DecayMask)) >> DecayShift
		r := (r0 + (r0 + ((-r0)>>31)&DecayMask)) >> DecayShift
		l0 -= l
		r0 -= r
		b.S284[2*i] += l
		b.S284[2*i+1] += r
	}

	*offL, *offR = l0, r0
}

func (b *Buffer) StereoFill(offL, offR *Fp284, sampleCount int) {
	l0, r0 := *offL, *offR

	if l0 == 0 && r0 == 0 {
		clear(b.S284[:2*sampleCount])

		return
	}

	for i := range b.S284[:sampleCount] {
		// Apply simple exponential decay to left and right offsets
		l := (l0 + (l0 + ((-l0)>>31)&DecayMask)) >> DecayShift
		r := (r0 + (r0 + ((-r0)>>31)&DecayMask)) >> DecayShift
		l0 -= l
		r0 -= r
		b.S284[2*i] = l
		b.S284[2*i+1] = r
	}

	*offL, *offR = l0, r0
}

func (b *Buffer) All() iter.Seq2[int, Fp284] {
	return slices.All(b.S284)
}

func (b *Buffer) Values() iter.Seq[Fp284] {
	return slices.Values(b.S284)
}

func (b *Buffer) AllMono() iter.Seq2[int, Fp284] {
	return func(yield func(int, Fp284) bool) {
		next, stop := iter.Pull2(b.All())

		defer stop()

		for {
			i, l, okL := next()
			_, r, okR := next()

			if !okL || !okR {
				return
			}

			i /= 2
			c := (l + r) / 2

			if !yield(i, c) {
				return
			}
		}
	}
}

func (b *Buffer) AllInterleaved(o *Buffer) iter.Seq2[int, Fp284] {
	return func(yield func(int, Fp284) bool) {
		next, stop := iter.Pull2(xiter.Zip2(b.Values(), o.Values()))

		defer stop()

		for i := 0; ; i++ {
			front, rear, ok := next()

			if !ok {
				return
			}

			for _, v := range []Fp284{front, rear} {
				if !yield(i, v) {
					return
				}
			}
		}
	}
}

func (b *Buffer) ToPCM8(buf []byte) int {
	return Convert284To8(buf, b.S284)
}

func (b *Buffer) ToPCM16(buf []byte) int {
	return Convert284To16(buf, b.S284)
}

func (b *Buffer) ToPCM24(buf []byte) int {
	return Convert284To24(buf, b.S284)
}

func (b *Buffer) ToPCM32(buf []byte) int {
	return Convert284To32(buf, b.S284)
}

// InterleaveFrontRear interleaves two Fp284 buffers (front and rear) into a single stereo buffer.
// The result is [front[0], rear[0], front[1], rear[1], ...].
func (b *Buffer) Interleave(o *Buffer) *Buffer {
	front, rear := b.S284, o.S284
	// Determine the length of the shorter buffer
	n := min(len(front), len(rear))
	out := make([]Fp284, n*2)

	// Interleave the samples
	for i := range n {
		out[i*2] = front[i]
		out[i*2+1] = rear[i]
	}

	return &Buffer{S284: out}
}
