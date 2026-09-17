// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mp3

import (
	"fmt"
	"io"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
	gomp3 "github.com/hajimehoshi/go-mp3"
)

type Decoder gomp3.Decoder

func Decode(in io.Reader) (*Decoder, error) {
	dec, err := gomp3.NewDecoder(in)

	r := (*Decoder)(dec)

	err = mkError(ErrDecode, err)

	return r, err
}

func (r *Decoder) Read(p []byte) (int, error) {
	mr := (*gomp3.Decoder)(r)
	n, err := mr.Read(p)

	return n, mkError(ErrDecode, err)
}

func (r *Decoder) ReadAt(p []byte, off int64) (n int, err error) {
	defer func() {
		err = mkError(ErrDecode, err)
	}()

	off0, err := r.Seek(0, io.SeekCurrent)

	if err != nil {
		return
	}

	defer func() {
		_, err = r.Seek(off0, io.SeekStart)
	}()

	nwant := len(p)
	if nwant > int(r.Size()-off) {
		p = p[:r.Size()-off]
	}

	_, err = r.Seek(off, io.SeekStart)

	if err != nil {
		return 0, mkError(ErrSeek, err)
	}

	n, err = r.Read(p)

	if n < nwant && err == nil {
		err = io.EOF
	}

	return
}

func (r *Decoder) Seek(offset int64, whence int) (int64, error) {
	mr := (*gomp3.Decoder)(r)
	n, err := mr.Seek(offset, whence)

	return n, mkError(ErrSeek, err)
}

func (r *Decoder) Reset() (err error) {
	_, err = r.Seek(0, io.SeekStart)
	if err != nil {
		return mkError(ErrSeek, err)
	}

	return nil
}

func (r *Decoder) Format() pcm.Format {
	mr := (*gomp3.Decoder)(r)
	sr := mr.SampleRate()

	f, _ := pcm.NewFormat('s', 16, 2, sr)

	return f
}

func (r *Decoder) Size() int64 {
	mr := (*gomp3.Decoder)(r)
	return mr.Length()
}

func (r *Decoder) Duration() time.Duration {
	return r.Format().Duration(r.Size())
}

func (r *Decoder) String() string {
	sz := r.Size() / kB

	return fmt.Sprintf(
		"MP3{%v, %v, %dkB}",
		r.Format(), r.Duration(), sz,
	)
}

const kB = 1024
