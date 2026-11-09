// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package mid

// frac represents a rational number
type frac struct {
	num, den int64
}

func newFrac(n, d int64) frac {
	return frac{num: n, den: d}.reduce()
}

func (f frac) eval() float64 {
	return float64(f.num) / float64(f.den)
}

func (f frac) mul(o frac) frac {
	return frac{num: f.num * o.num, den: f.den * o.den}.reduce()
}

func (f frac) reduce() frac {
	gcd := func(a, b int64) int64 {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}

	g := gcd(f.num, f.den)
	return frac{num: f.num / g, den: f.den / g}
}
