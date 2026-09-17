// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sample

import "github.com/erik-adelbert/duh/internal/tracker/components/core"

type Infos struct {
	Name    string
	Length  uint32
	Loop    core.Span[uint32]
	Sustain core.Span[uint32]
}

type Data []int8

type Playback struct {
	Flags
	C4Speed  uint32
	FineTune int16
	Vibrato  struct {
		Type  uint8
		Sweep uint8
		Depth uint8
		Rate  uint8
	}
}

type Volume struct {
	Global uint8
	Value  uint8
	Pan    uint8
}
