// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wav

import (
	"io"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Format = pcm.Format

func isWAV(f Format) bool {

	switch f.Tag() {
	case pcm.Float:
		return true
	case pcm.UnsignedIntLE, pcm.UnsignedIntBE:
		return f.BitDepth() == 8
	case pcm.SignedIntLE, pcm.SignedIntBE:
		return f.BitDepth() != 8
	}

	return false
}

func MkFormat(tag rune, channelCount, bitDepth, sampleRate int) (f Format, err error) {
	f, err = pcm.NewFormat(tag, bitDepth, channelCount, sampleRate)

	if err != nil || !isWAV(f) {
		return Format{}, ErrFormat
	}

	return
}

func ParseFormat(s string) (f Format, err error) {
	f, err = pcm.ParseFormat(s)

	if err != nil || !isWAV(f) {
		return Format{}, ErrFormat
	}

	return
}

type PCMStream interface {
	io.Seeker
	Format() Format
}
