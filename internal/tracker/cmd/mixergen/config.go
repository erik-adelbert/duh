// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import "iter"

type BitDepth uint8

const (
	Bit8  BitDepth = 8
	Bit16 BitDepth = 16
)

type Channels uint8

const (
	FastMono Channels = iota
	Mono
	Stereo
)

type Interp uint8

const (
	InterpNone Interp = iota
	InterpLinear
	InterpSpline
	InterpFIR
)

type Ramp uint8

const (
	RampNone Ramp = iota
	RampSlow
	RampFast
)

type Filter bool

type Config struct {
	Package  string
	Variants iter.Seq[Variant]
}
