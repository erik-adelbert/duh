// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mp3

import (
	"io"
	"time"

	shmp3 "github.com/braheezy/shine-mp3/pkg/mp3"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Encoder struct {
	pcm.Format
	*shmp3.Encoder

	w io.Writer

	samples   []int16
	scratch32 []int32
	input     []byte

	pending struct {
		bytes   []byte
		samples []int16
	}
}

func Encode(out io.Writer, format pcm.Format) (w *Encoder, err error) {
	if format.BitDepth() != 16 || format.Tag() != 's' {
		err = mkError(ErrEncode, "unsupported PCM format")
		return
	}

	nchan := format.ChannelCount()

	// A sample count, not bytes; it must hold at least one frame or Write cannot make progress.
	bufsz := max(format.SampleCount(200*ms), int64(nchan))

	sr := format.SampleRate()

	enc := shmp3.NewEncoder(sr, nchan)

	samples := make([]int16, bufsz)
	scratch32 := make([]int32, bufsz)

	w = &Encoder{
		Format:    format,
		Encoder:   enc,
		w:         out,
		samples:   samples,
		scratch32: scratch32,
	}

	return
}

func (w *Encoder) Write(p []byte) (n int, err error) {
	in := p

	if len(w.pending.bytes) > 0 {
		in = append(w.input[:0], w.pending.bytes...)
		in = append(in, p...)
		w.input = in
	}

	framesz := int(w.FrameSize())
	alignedsz := len(in) / framesz * framesz

	w.pending.bytes = append(w.pending.bytes[:0], in[alignedsz:]...)

	// DecodeS16 stops at the capacity of w.samples, so feed it one buffer's worth at a time.
	chunksz := len(w.samples) / w.Format.ChannelCount() * framesz

	for todo := in[:alignedsz]; len(todo) > 0; {
		chunk := todo[:min(chunksz, len(todo))]
		todo = todo[len(chunk):]

		nsample := w.DecodeS16(w.samples, chunk, w.scratch32)
		w.pending.samples = append(w.pending.samples, w.samples[:nsample]...)

		if err = w.encode(); err != nil {
			return len(p), mkError(ErrEncode, err)
		}
	}

	return len(p), nil
}

func (w *Encoder) encode() error {
	spf := w.samplePerFrame()
	nused := 0

	for len(w.pending.samples)-nused >= spf {
		data, n := w.EncodeBufferInterleaved(
			w.pending.samples[nused : nused+spf],
		)

		if err := writeAll(w.w, data[:n]); err != nil {
			return err
		}

		nused += spf
	}

	if nused > 0 {
		nrem := copy(w.pending.samples, w.pending.samples[nused:])
		w.pending.samples = w.pending.samples[:nrem]
		// clear(w.pending.samples[nrem:])
	}

	return nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)

		if n > 0 {
			p = p[n:]
		}

		switch {
		case err != nil:
			return err
		case n == 0:
			return io.ErrShortWrite
		}
	}
	return nil
}

func (w *Encoder) Close() error {
	if len(w.pending.bytes) > 0 {
		return mkError(ErrEncode, "incomplete PCM frame at end of input")
	}

	if len(w.pending.samples) == 0 {
		return nil
	}

	spf := w.samplePerFrame()

	rem := len(w.pending.samples)

	if cap(w.pending.samples) < spf {
		padded := make([]int16, spf)
		copy(padded, w.pending.samples)
		w.pending.samples = padded
	}

	w.pending.samples = w.pending.samples[:spf]
	clear(w.pending.samples[rem:])

	if err := w.encode(); err != nil {
		return mkError(ErrEncode, err)
	}

	w.pending.samples = nil

	return nil
}

func (w *Encoder) samplePerFrame() int {
	enc := w.Encoder
	return int(576 * enc.Mpeg.GranulesPerFrame * enc.Wave.Channels)
}

const ms = time.Millisecond
