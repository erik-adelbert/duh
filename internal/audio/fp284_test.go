// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"bytes"
	"testing"
)

func TestInt284(t *testing.T) {
	tests := []struct {
		in   Fp284
		want int32
	}{
		{0, 0},
		{16, 1},
		{255, 15},
		{-16, -1},
		{256, 16},
	}

	for _, tt := range tests {
		got := Int284(tt.in)

		if got != tt.want {
			t.Errorf("Int284(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFrac284(t *testing.T) {
	tests := []struct {
		in   Fp284
		want int32
	}{
		{0, 0},
		{15, 15},
		{16, 0},
		{31, 15},
		{255, 15},
	}

	for _, tt := range tests {
		got := Frac284(tt.in)

		if got != tt.want {
			t.Errorf("Frac284(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestFp284ToPCM8(t *testing.T) {
	tests := []struct {
		in   Fp284
		want PCM8
	}{
		{0, 128},
		{256, 128},
		{-256, 127},
		{Max284, 255},
		{Min284, 0},
		{131072, 129},  // +2.0 clamped to +1.0
		{-131072, 126}, // -2.0 clamped to -1.0
	}

	for _, tt := range tests {
		got := Fp284ToPCM8(tt.in)

		if got != tt.want {
			t.Errorf("Fp284ToPCM8(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestConvert284To8(t *testing.T) {
	src := []Fp284{
		0,
		Max284, // nominal +1.0
		Min284, // nominal -1.0
	}
	dst := make([]byte, 3)
	n := Convert284To8(dst, src)

	if n != 3 {
		t.Errorf("Convert284To8 n = %d, want 3", n)
	}

	want := []byte{128, 255, 0} // mid, +1, -1

	if !bytes.Equal(dst, want) {
		t.Errorf("Convert284To8 dst = %v, want %v", dst, want)
	}
}

func TestConvert284To16(t *testing.T) {
	src := []Fp284{0, 1 << 16}
	dst := make([]byte, 4)
	n := Convert284To16(dst, src)

	if n != 2 {
		t.Errorf("Convert284To16 n = %d, want 2", n)
	}

	want := []byte{0, 0, 128, 0}

	if !bytes.Equal(dst, want) {
		t.Errorf("Convert284To16 dst = %v, want %v", dst, want)
	}
}

func TestConvert284To24(t *testing.T) {
	src := []Fp284{0, Max284}
	dst := make([]byte, 6)
	n := Convert284To24(dst, src)

	if n != 2 {
		t.Errorf("Convert284To24 n = %d, want 2", n)
	}

	want := []byte{0, 0, 0, 255, 255, 127}

	if !bytes.Equal(dst, want) {
		t.Errorf("Convert284To24 dst = %v, want %v", dst, want)
	}
}

func TestConvert284To32(t *testing.T) {
	src := []Fp284{0, 1 << 16}
	dst := make([]byte, 8)
	n := Convert284To32(dst, src)

	if n != 2 {
		t.Errorf("Convert284To32 n = %d, want 2", n)
	}

	want := []byte{0, 0, 0, 0, 0, 0, 128, 0}

	if !bytes.Equal(dst, want) {
		t.Errorf("Convert284To32 dst = %v, want %v", dst, want)
	}
}

func TestConvert8To284(t *testing.T) {
	src := []byte{128, 129, 127}
	dst := make([]Fp284, 3)
	n := Convert8To284(dst, src)

	if n != 3 {
		t.Errorf("Convert8To284 n = %d, want 3", n)
	}

	// Correct Fp284 values after PCM8 -> Fp284
	want := []Fp284{0, 131072, -131072}

	for i := range dst {
		if dst[i] != want[i] {
			t.Errorf("Convert8To284 dst[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
}

func TestConvert16To284(t *testing.T) {
	src := []byte{0, 0, 1, 0}
	dst := make([]Fp284, 2)
	n := Convert16To284(dst, src)

	if n != 2 {
		t.Errorf("Convert16To284 n = %d, want 2", n)
	}

	want := []Fp284{0, 512}

	for i := range dst {
		if dst[i] != want[i] {
			t.Errorf("Convert16To284 dst[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
}

func TestConvert24To284(t *testing.T) {
	src := []byte{0, 0, 0, 1, 0, 0}
	dst := make([]Fp284, 2)
	n := Convert24To284(dst, src)

	if n != 2 {
		t.Errorf("Convert24To284 n = %d, want 2", n)
	}

	want := []Fp284{0, 0}

	for i := range dst {
		if dst[i] != want[i] {
			t.Errorf("Convert24To284 dst[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
}

func TestConvert32To284(t *testing.T) {
	src := []byte{0, 0, 0, 0, 1, 0, 0, 0}
	dst := make([]Fp284, 2)
	n := Convert32To284(dst, src)

	if n != 2 {
		t.Errorf("Convert32To284 n = %d, want 2", n)
	}

	want := []Fp284{0, 0}

	for i := range dst {
		if dst[i] != want[i] {
			t.Errorf("Convert32To284 dst[%d] = %d, want %d", i, dst[i], want[i])
		}
	}
}
