// duh tinyfft package
//
// # Based on code by Ryuhei Mori
//
// Copyright (c) 2017, Ryuhei Mori
// Copyright (c) 2026, Erik Adelbert
//
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package tinyfft implements in-place Cooley-Tukey Radix-4 Fast Fourier Transform (FFT) in Go.
package tinyfft

func Example() {
	tf, _ := NewTinyFFT(4)

	x := []complex128{1, 1, 1, 1, 0, 0, 0, 0}

	_ = tf.FFT(x) // Compute the FFT of x in-place

	_ = tf.Natural(x) // Rearrange the result to natural order
}
