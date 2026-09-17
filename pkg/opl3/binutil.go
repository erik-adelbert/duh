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

import "golang.org/x/exp/constraints"

// has checks if the given integer x has the specified bitmask set.
func has[T constraints.Integer](x T, mask T) bool {
	return x&mask != 0
}

// bit returns the value of the nth bit of the given integer x.
func bit[T constraints.Integer](n uint, x T) uint8 {
	return uint8((x >> n) & 1)
}

// hi6 returns the high 6 bits of the given byte x.
func hi6(x uint8) uint8 {
	return lo6(x >> 2)
}

// hi4 returns the high 4 bits of the given byte x.
func hi4(x uint8) uint8 {
	return lo4(x >> 4)
}

// hi2 returns the high 2 bits of the given byte x.
func hi2(x uint8) uint8 {
	return lo2(x >> 6)
}

// lo6 returns the low 6 bits of the given byte x.
func lo6(x uint8) uint8 {
	return x & 0x3f
}

// lo4 returns the low 4 bits of the given byte x.
func lo4(x uint8) uint8 {
	return x & 0xf
}

// lo5 returns the low 5 bits of the given byte x.
func lo3(x uint8) uint8 {
	return x & 0x7
}

// lo2 returns the low 2 bits of the given byte x.
func lo2(x uint8) uint8 {
	return x & 0x3
}
