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
}

func Encode(out io.Writer, format pcm.Format) (w *Encoder, err error) {
	if format.BitDepth() != 16 || format.Tag() != 's' {
		err = mkError(ErrEncode, "unsupported PCM format")
		return
	}

	var bufferSize = format.BufferSize(200 * ms)

	sr := format.SampleRate()
	nchan := format.ChannelCount()

	enc := shmp3.NewEncoder(sr, nchan)

	samples := make([]int16, bufferSize)
	scratch32 := make([]int32, bufferSize)

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
	n = w.DecodeS16(w.samples, p, w.scratch32)

	err = w.Encoder.Write(w.w, w.samples[:n])

	if err != nil {
		err = mkError(ErrEncode, err)
		return
	}

	return
}

const ms = time.Millisecond
