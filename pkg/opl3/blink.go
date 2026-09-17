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

// blink is a bidirectional link between two types, A and B.
// It allows for easy access to both types from either side of the link.
type blink[A, B any] struct { // in your area
	dst *A
	src *B
}

func (l blink[A, B]) A() *A {
	return l.dst
}

func (l blink[A, B]) B() *B {
	return l.src
}
