// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package aac

import (
	"fmt"
	"io"

	"github.com/erik-adelbert/duh/pkg/pcm"
	aacpcm "github.com/tphakala/go-aac/pcm"
)

type Decoder aacpcm.Decoder

func Decode(r io.Reader) (*Decoder, error) {
	dec, err := aacpcm.NewDecoder(r)
	if err != nil {
		return nil, err
	}

	return (*Decoder)(dec), nil
}

func (d *Decoder) Read(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrDecode, err)
	}()

	ad := (*aacpcm.Decoder)(d)

	return ad.Read(p)
}

func (d *Decoder) Format() pcm.Format {
	ad := (*aacpcm.Decoder)(d)

	infos := ad.Info()

	f, _ := pcm.NewFormat(
		pcm.SignedIntLE, 16, infos.Channels, infos.SampleRate,
	)

	return f
}

func (d *Decoder) String() string {
	ad := (*aacpcm.Decoder)(d)
	return fmt.Sprintf("AAC{%s %s}", d.Format(), ad.Info().Profile)
}
