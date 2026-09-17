// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import "testing"

func TestChannelInit_IdentityRate(t *testing.T) {
	var c channel

	n, err := c.init(44_100, 44_100, 64)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	if c.ρ != One {
		t.Fatalf("ρ = %d, want %d", c.ρ, One)
	}

	if n != 64 {
		t.Fatalf("sample count = %d, want 64 for identity rate", n)
	}
}

func TestChannelInit_ResamplingState(t *testing.T) {
	var c channel

	inputN := 64
	n, err := c.init(44_100, 48_000, inputN)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	if c.ρ == One {
		t.Fatalf("ρ should not be unity for differing rates")
	}

	if c.w <= 0 {
		t.Fatalf("w = %d, want > 0", c.w)
	}

	if c.i != c.w {
		t.Fatalf("buffer index i = %d, want w = %d", c.i, c.w)
	}

	if c.t != uint32(c.w)<<Np {
		t.Fatalf("t = %d, want %d", c.t, uint64(c.w)<<Np)
	}

	wantLen := c.w*2 + inputN
	if len(c.x) != int(wantLen) {
		t.Fatalf("len(x) = %d, want %d", len(c.x), wantLen)
	}

	if n <= 0 {
		t.Fatalf("sample count = %d, want > 0", n)
	}
}

func TestChannelResample_IdentityCopiesMin(t *testing.T) {
	c := channel{ρ: One}
	in := []int32{1, 2, 3, 4, 5}
	out := make([]int32, 3)

	n := c.resample(out, in, 5)

	if n != 3 {
		t.Fatalf("resample count = %d, want 3", n)
	}

	want := []int32{1, 2, 3}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("out[%d] = %d, want %d", i, out[i], want[i])
		}
	}
}

func TestChannelResample_ReportsUnderrun(t *testing.T) {
	var c channel

	_, err := c.init(44_100, 48_000, 16)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}

	in := []int32{1}
	out := make([]int32, 16)

	n := c.resample(out, in, len(in))
	if n != 0 {
		t.Fatalf("resample return = %d, want 0 for underrun", n)
	}
}
