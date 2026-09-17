// duh OPL3 emulator package
//
// Based on Nuked OPL3 by Nuke.YKT.
//
// Original:
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package opl3

import (
	"math"
)

func lerp(old, nxt int16, frac, denom int32) int16 {
	if denom <= 0 {
		return old
	}

	α64, ω64 := int64(old), int64(nxt)
	f64, d64 := int64(frac), int64(denom)

	x64 := α64*(d64-f64) + ω64*f64

	return int16(x64 / d64)
}

func linexp(x uint32) uint16 {
	const (
		tableMask        = 0xff
		scaleMax  uint32 = 0x1fff
	)

	x = min(x, scaleMax)

	i, shift := x&tableMask, x>>8

	return expROM[i] >> shift
}

func clip16(x int32) int16 {
	switch {
	case x > math.MaxInt16:
		return math.MaxInt16
	case x < math.MinInt16:
		return math.MinInt16
	}

	return int16(x)
}
