// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dsp

import "github.com/erik-adelbert/duh/internal/audio"

const ReverbBufferSize = 64

type Reverb struct {
	LowPass [2]*Buffer
	Reverb  [4]*Buffer
	Size    [4]int32
	BufIdx  [4]int32
	// closer  func()
}

type Buffer = audio.Buffer
