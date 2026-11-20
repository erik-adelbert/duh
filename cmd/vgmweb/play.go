// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"

	"github.com/erik-adelbert/duh/internal/webaudio"
)

const (
	audioChunkBytes     = webaudio.ChunkBytes
	audioBytesPerSecond = 44100 * 4
)

func play(in io.Reader, audio webaudio.Output) error {
	return webaudio.Play(in, audio, audioBytesPerSecond)
}
