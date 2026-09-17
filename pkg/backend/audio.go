// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Audio interface {
	BitDepth() int
	ChannelCount() int
	Close()
	Duration() (time.Duration, error)
	Format() pcm.Format
	IsPlaying() bool
	Pause()
	Play()
	Position() int64
	SampleRate() int
	Seek(offset int64, whence int) (int64, error)
	SetVolume(volume float64)
}
