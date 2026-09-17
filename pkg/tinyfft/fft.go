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

package tinyfft

import (
	"errors"
	"math"
	"math/bits"
	"math/cmplx"
)

var ErrWrongSize = errors.New("input size error")

// TinyFFT represents a Fast Fourier Transform instance with precomputed twiddle factors.
type TinyFFT struct {
	w []complex128 // Precomputed twiddle factors for the FFT
	k uint         // log2 size of the FFT
	n int          // size of the FFT
}

// NewTinyFFT creates a new TinyFFT instance for the given log2 size.
func NewTinyFFT(logSize int) (*TinyFFT, error) {
	k := uint(logSize)

	n := 1 << k

	w := make([]complex128, n/2)

	θ := -2 * math.Pi / float64(n)

	for i, j := 1, n/4; j != 0; i, j = i*2, j/2 {
		j := float64(j)
		w[i] = cmplx.Exp(complex(0, j*θ))
	}

	w = twiddles(w, 0, n/4, 1+0i)

	return &TinyFFT{w: w, k: k, n: n}, nil
}

// Log2 returns the log2 size of the FFT.
func (tf *TinyFFT) Log2() int {
	return int(tf.k)
}

// Size returns the size of the FFT, which is 2^k.
func (tf *TinyFFT) Size() int {
	return int(tf.n)
}

// Normalize scales the elements of x by 1/n, where n is the size of the FFT.
// This should be called after IFFT to get the correct inverse transform results.
func (tf *TinyFFT) Normalize(x []complex128) {
	n := float64(tf.Size())

	for i := range x {
		x[i] /= complex(n, 0)
	}
}

// Natural rearranges the elements of x in-place to be in natural order
// (bit-reversed order to normal order). The length of x must match
// the size of the FFT.
func (tf *TinyFFT) Natural(x []complex128) error {
	if len(x) != tf.Size() {
		return ErrWrongSize
	}

	n := tf.Size()
	for i := range n {
		j := bitReverse(i, tf.k)
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}

	return nil
}

// bitReverse computes the bit-reversal of x with respect to k bits.
func bitReverse(x int, k uint) int {
	if k <= 32 {
		return int(bits.Reverse32(uint32(x)) >> (32 - k))
	}
	return int(bits.Reverse64(uint64(x)) >> (64 - k))
}

// twiddles recursively fills the twiddle factor slice w with the appropriate values for the FFT.
func twiddles(w []complex128, i, b int, z complex128) []complex128 {
	if b == 0 {
		w[i] = z
		return w
	}

	w = twiddles(w, i, b/2, z)
	w = twiddles(w, i|b, b/2, z*w[b])

	return w
}

// FFT computes the forward Fast Fourier Transform of the input slice x in-place.
// The length of x must match the size of the FFT.
// Result is in bit-reversed order.
func (tf *TinyFFT) FFT(x []complex128) error {
	if len(x) != tf.Size() {
		return ErrWrongSize
	}

	k := tf.Log2()
	m := tf.Size()
	u, v := 1, m/4

	if k&1 != 0 {
		for i := range m / 2 {
			x[i+m/2], x[i] = x[i]-x[i+m/2], x[i]+x[i+m/2]
		}

		u *= 2
		v /= 2
	}

	for i := k &^ 1; i > 0; i -= 2 {
		for jj := range u {
			w1 := tf.w[jj*2]
			w2 := tf.w[jj]
			w3 := w1 * w2

			j := jj << i
			jmax := j + v
			v2, v3 := 2*v, 3*v
			for ; j < jmax; j++ {
				x0 := x[j]
				x1 := w1 * x[j+v]
				x2 := w2 * x[j+v2]
				x3 := w3 * x[j+v3]

				x[j] = x0 + x1 + x2 + x3
				x[j+v] = x0 - x1 + x2 - x3
				x[j+v2] = x0 - x2 - 1i*(x1-x3)
				x[j+v3] = x0 - x2 + 1i*(x1-x3)
			}
		}

		u *= 4
		v /= 4
	}

	return nil
}

// IFFT computes the inverse Fast Fourier Transform of the input slice x in-place.
// The length of x must match the size of the FFT. Input is expected to be in bit-reversed order.
// After calling IFFT, you should call Normalize to scale the results by 1/n.
func (tf *TinyFFT) IFFT(x []complex128) error {
	if len(x) != tf.Size() {
		return ErrWrongSize
	}

	k := tf.Log2()
	m := tf.Size()
	u, v := m/4, 1

	for i := 2; i <= k; i += 2 {
		for jj := range u {
			w1 := cmplx.Conj(tf.w[jj*2])
			w2 := cmplx.Conj(tf.w[jj])
			w3 := w1 * w2

			j := jj << i
			jmax := j + v
			v2, v3 := 2*v, 3*v
			for ; j < jmax; j++ {
				x0 := x[j]
				x1 := x[j+v]
				x2 := x[j+v2]
				x3 := x[j+v3]

				x[j] = x0 + x1 + x2 + x3
				x[j+v] = w1 * (x0 - x1 + 1i*(x2-x3))
				x[j+v2] = w2 * (x0 + x1 - x2 - x3)
				x[j+v3] = w3 * (x0 - x1 - 1i*(x2-x3))
			}
		}

		u /= 4
		v *= 4
	}

	if k&1 != 0 {
		for i := range m / 2 {
			x[i+m/2], x[i] = x[i]-x[i+m/2], x[i]+x[i+m/2]
		}
	}

	return nil
}
