// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import "fmt"

type streamInfos struct {
	header
	loop bool
}

func (f *streamInfos) String() string {
	return fmt.Sprintf(
		"ver: %s, clock: %dHz, looped: %v, length: %v",
		f.Ver, f.Clock, f.loop, f.Length,
	)
}

type header struct {
	Sig    [4]byte // "Vgm "
	Eof    offset
	Ver    bcd
	_      [16]byte
	Loop   offset
	_      [20]byte
	Cmd0   offset
	_      [36]byte
	Clock  uint32
	_      [120]byte
	Length int64 // Not from the VGM spec but still in reserved space.
}

type offset = uint32
