// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package main

import (
	"io"

	"github.com/erik-adelbert/duh/internal/webaudio"
)

const (
	// Keep in sync with the pool buffer size in docs/midweb/mid.html.
	audioChunkBytes     = webaudio.ChunkBytes
	audioBytesPerSecond = 48000 * 4
)

func play(in io.Reader, audio webaudio.Output) error {
	return webaudio.Play(in, audio, audioBytesPerSecond)
}
