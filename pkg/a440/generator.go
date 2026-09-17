// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"encoding/binary"
	"io"
	"iter"
	"math"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Generator struct {
	Format

	fq   Frequency
	pos  int64
	nout int64

	next func() (float64, bool)
	stop func()
}

func NewGenerator(f Format, duration time.Duration, freq Frequency, seq iter.Seq[float64]) *Generator {
	sr := f.SampleRate()

	frames := int64(duration.Seconds() * float64(sr))

	next, stop := iter.Pull(seq)

	return &Generator{
		Format: f,

		fq:   freq,
		nout: frames,

		next: next,
		stop: stop,
	}
}

func (r *Generator) Read(p []byte) (int, error) {
	if r.pos >= r.nout {
		return 0, io.EOF
	}

	spsz := r.BitDepth() / 8        // bytes per sample
	frsz := spsz * r.ChannelCount() // bytes per frame

	nframe := len(p) / frsz
	if nframe == 0 {
		return 0, nil
	}

	rem := r.nout - r.pos
	if int64(nframe) > rem {
		nframe = int(rem)
	}

	n := nframe * frsz

	next := r.next

	putuint16 := ble.PutUint16
	putuint32 := ble.PutUint32

	tag := r.Tag()

	if tag == pcm.UnsignedIntBE || tag == pcm.SignedIntBE {
		putuint16 = bbe.PutUint16
		putuint32 = bbe.PutUint32
	}

	for i := range nframe {
		x, ok := next()

		if !ok {
			return n, io.EOF
		}

		max := (int64(1) << (r.BitDepth() - 1)) - 1
		xnorm := x * float64(max)

		sample := int32(math.Round(xnorm))

		for ch := range r.ChannelCount() {
			off := i*frsz + ch*spsz

			switch r.BitDepth() {
			case 8:
				p[off] = byte(int8(sample + 128))

			case 16:
				putuint16(
					p[off:off+2],
					uint16(int16(sample)),
				)

			case 24:
				v := int32(sample)
				p[off+0] = byte(v)
				p[off+1] = byte(v >> 8)
				p[off+2] = byte(v >> 16)

				if tag == pcm.UnsignedIntBE || tag == pcm.SignedIntBE {
					p[off+0], p[off+2] = p[off+2], p[off+0]
				}

			case 32:
				u32 := uint32(sample)

				if tag == pcm.Float {
					u32 = math.Float32bits(float32(x))
				}

				putuint32(p[off:off+4], u32)
			}
		}
	}

	r.pos += int64(nframe)

	return n, nil
}

func (r *Generator) Close() error {
	r.stop()

	return nil
}

type Format = pcm.Format

var ble = binary.LittleEndian
var bbe = binary.BigEndian
