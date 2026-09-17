// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package aac

import (
	"io"

	aacpcm "github.com/tphakala/go-aac/pcm"
)

type Encoder aacpcm.Encoder

func Encode(in io.Writer, f Format) (w *Encoder, err error) {
	var enc *aacpcm.Encoder

	enc, err = aacpcm.NewEncoder(in, f.Config)

	w = (*Encoder)(enc)
	err = mkError(ErrEncode, err)

	return
}

func (w *Encoder) Write(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrEncode, err)
	}()

	ac := (*aacpcm.Encoder)(w)
	return ac.Write(p)
}

func (w *Encoder) Close() error {
	ac := (*aacpcm.Encoder)(w)
	return mkError(ErrEncode, ac.Close())
}
