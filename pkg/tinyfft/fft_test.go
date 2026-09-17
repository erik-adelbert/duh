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
	"fmt"
	"math"
	"math/cmplx"
	"testing"
)

const ε = 1e-9

func almostEqual(a, b complex128, ε float64) bool {
	return cmplx.Abs(a-b) < ε
}

func slicesAlmostEqual(a, b []complex128, ε float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !almostEqual(a[i], b[i], ε) {
			return false
		}
	}
	return true
}

func TestNewTinyFFT_Size(t *testing.T) {
	tf, _ := NewTinyFFT(2)
	if tf.Size() != 4 {
		t.Errorf("expected size 4, got %d", tf.Size())
	}
	if tf.Log2() != 2 {
		t.Errorf("expected log 2, got %d", tf.Log2())
	}
}

func TestFFT_WrongSize(t *testing.T) {
	tf, _ := NewTinyFFT(2)
	input := []complex128{1, 2, 3}
	err := tf.FFT(input)
	if err != ErrWrongSize {
		t.Errorf("expected ErrWrongSize, got %v", err)
	}
}

func TestIFFT_WrongSize(t *testing.T) {
	tf, _ := NewTinyFFT(2)
	input := []complex128{1, 2, 3}
	err := tf.IFFT(input)
	if err != ErrWrongSize {
		t.Errorf("expected ErrWrongSize, got %v", err)
	}
}

func TestNormalize(t *testing.T) {
	expected := []complex128{0.5, 1, 1.5, 2}

	tf, _ := NewTinyFFT(2)

	x := []complex128{2, 4, 6, 8}
	tf.Normalize(x)

	if !slicesAlmostEqual(x, expected, ε) {
		t.Errorf("Normalize failed, got %v, want %v", x, expected)
	}
}

func TestNatural(t *testing.T) {
	expected := []complex128{1, 3, 2, 4}

	tf, _ := NewTinyFFT(2)

	x := []complex128{1, 2, 3, 4}

	err := tf.Natural(x)
	if err != nil {
		t.Fatalf("Natural error: %v", err)
	}

	if !slicesAlmostEqual(x, expected, ε) {
		t.Errorf("Natural failed, got %v, want %v", x, expected)
	}
}

func TestFFT_KnownInput(t *testing.T) {
	tests := []struct {
		k        int
		input    []complex128
		expected []complex128
	}{
		{
			k:        2,
			input:    []complex128{1, 0, 0, 0},
			expected: []complex128{1, 1, 1, 1},
		},
		{
			k:        3,
			input:    []complex128{1, 0, 0, 0, 0, 0, 0, 0},
			expected: []complex128{1, 1, 1, 1, 1, 1, 1, 1},
		},
	}

	for _, tc := range tests {
		tf, _ := NewTinyFFT(tc.k)
		x := append([]complex128(nil), tc.input...)

		err := tf.FFT(x)
		if err != nil {
			t.Fatalf("FFT error (k=%d): %v", tc.k, err)
		}

		if !slicesAlmostEqual(x, tc.expected, ε) {
			t.Errorf("FFT(%v) (k=%d) = %v, want %v", tc.input, tc.k, x, tc.expected)
		}
	}
}

func TestIFFT_KnownInput(t *testing.T) {
	tests := []struct {
		k        int
		input    []complex128
		expected []complex128
	}{
		{
			k:        2,
			input:    []complex128{1, 1, 1, 1},
			expected: []complex128{1, 0, 0, 0},
		},
		{
			k:        3,
			input:    []complex128{1, 1, 1, 1, 1, 1, 1, 1},
			expected: []complex128{1, 0, 0, 0, 0, 0, 0, 0},
		},
	}

	for _, tc := range tests {
		tf, _ := NewTinyFFT(tc.k)
		x := append([]complex128(nil), tc.input...)

		err := tf.IFFT(x)
		if err != nil {
			t.Fatalf("IFFT error (k=%d): %v", tc.k, err)
		}
		tf.Normalize(x)

		if !slicesAlmostEqual(x, tc.expected, ε) {
			t.Errorf("IFFT(%v) (k=%d) = %v, want %v", tc.input, tc.k, x, tc.expected)
		}
	}
}

func TestFFTRoundTrip(t *testing.T) {
	const k = 8 // size = 256
	tf, _ := NewTinyFFT(k)

	n := tf.Size()
	x := make([]complex128, n)

	// deterministic, non-trivial signal
	for i := range x {
		x[i] = complex(
			float64((i*i)%17),
			float64((i*7)%13),
		)
	}

	src := append([]complex128(nil), x...)

	if err := tf.FFT(x); err != nil {
		t.Fatal(err)
	}
	if err := tf.IFFT(x); err != nil {
		t.Fatal(err)
	}
	tf.Normalize(x)

	for i := range x {
		if !almostEqual(x[i], src[i], ε) {
			t.Fatalf("mismatch at %d: got %v, want %v", i, x[i], src[i])
		}
	}
}

func TestTwiddles(t *testing.T) {
	// Test that twiddles returns a slice with the correct length and non-zero values
	m := 4
	w := make([]complex128, m/2)
	θ := -2 * math.Pi / float64(m)

	for i, j := 1, m/4; j != 0; i, j = i*2, j/2 {
		jf := float64(j)
		w[i] = cmplx.Exp(complex(0, θ*jf))
	}

	w = twiddles(w, 0, m/4, 1+0i)
	nonZero := false
	for _, v := range w {
		if v != 0 {
			nonZero = true
			break
		}
	}

	if !nonZero {
		t.Errorf("twiddles returned all zeros")
	}
}

func BenchmarkFFT(b *testing.B) {
	ks := []int{2, 3, 8, 10} // k=10 -> size 1024, matches the spectrum analyzer

	for _, k := range ks {
		b.Run("k="+fmt.Sprint(k), func(b *testing.B) {
			tf, _ := NewTinyFFT(k)
			n := tf.Size()

			x := make([]complex128, n)
			for i := range x {
				x[i] = complex(float64(i), float64(i%3))
			}

			for b.Loop() {
				copyBuf := append([]complex128(nil), x...)
				_ = tf.FFT(copyBuf)
			}
		})
	}
}

func BenchmarkIFFT(b *testing.B) {
	ks := []int{2, 3, 8, 10}

	for _, k := range ks {
		b.Run("k="+fmt.Sprint(k), func(b *testing.B) {
			tf, _ := NewTinyFFT(k)
			n := tf.Size()

			x := make([]complex128, n)
			for i := range x {
				x[i] = complex(float64(i), float64(i%3))
			}

			_ = tf.FFT(x)

			for b.Loop() {
				copyBuf := append([]complex128(nil), x...)
				_ = tf.IFFT(copyBuf)
			}
		})
	}
}

func BenchmarkFFTRoundTrip(b *testing.B) {
	ks := []int{2, 3, 8, 10}
	for _, k := range ks {
		b.Run("k="+fmt.Sprint(k), func(b *testing.B) {
			tf, _ := NewTinyFFT(k)
			n := tf.Size()

			x := make([]complex128, n)
			for i := range x {
				x[i] = complex(float64(i), float64(i%3))
			}

			for b.Loop() {
				copyBuf := append([]complex128(nil), x...)
				_ = tf.FFT(copyBuf)
				_ = tf.IFFT(copyBuf)
				tf.Normalize(copyBuf)
			}
		})
	}
}
