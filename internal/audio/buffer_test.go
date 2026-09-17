// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"bytes"
	"testing"
)

func TestGetPutBuffer(t *testing.T) {
	buf := GetBuffer(10)

	defer PutBuffer(buf)

	if buf.Len() != 10 {
		t.Errorf("expected length 10, got %d", buf.Len())
	}
}

func TestNewBufferVariants(t *testing.T) {
	// Test NewBuffer() with no arguments
	buf0, closer0 := NewBuffer()

	defer closer0()

	if buf0.Len() != 0 {
		t.Errorf("NewBuffer(): expected len=0, got len=%d", buf0.Len())
	}

	// Test NewBuffer(size)
	buf1, closer1 := NewBuffer(7)

	defer closer1()

	if buf1.Len() != 7 || buf1.Cap() < 7 {
		t.Errorf("NewBuffer(7): expected len=7, cap>=7, got len=%d, cap=%d", buf1.Len(), buf1.Cap())
	}

	// Test NewBuffer(size, cap)
	buf2, closer2 := NewBuffer(3, 10)

	defer closer2()

	if buf2.Len() != 3 || buf2.Cap() < 10 {
		t.Errorf("NewBuffer(3,10): expected len=3, cap>=10, got len=%d, cap=%d", buf2.Len(), buf2.Cap())
	}

	// Ensure buffer is zeroed
	for i, v := range buf2.S284 {
		if v != 0 {
			t.Errorf("NewBuffer(3,10): expected zeroed buffer at index %d, got %d", i, v)
		}
	}
}

func TestResize(t *testing.T) {
	buf := GetBuffer(5)

	defer PutBuffer(buf)

	oldCap := buf.Cap()

	buf.Resize(3)

	if buf.Len() != 3 {
		t.Errorf("expected length 3, got %d", buf.Len())
	}

	if buf.Cap() != oldCap {
		t.Errorf("expected cap %d, got %d", oldCap, buf.Cap())
	}
}

func TestSlice(t *testing.T) {
	buf := GetBuffer(6)

	defer PutBuffer(buf)

	for i := range buf.S284 {
		buf.S284[i] = Fp284(i)
	}

	slice := buf.Slice(2, 4)

	if slice.Len() != 2 || slice.S284[0] != 2 || slice.S284[1] != 3 {
		t.Errorf("unexpected slice: %+v", slice.S284)
	}
}

func TestClear(t *testing.T) {
	buf := GetBuffer(4)

	defer PutBuffer(buf)

	for i := range buf.S284 {
		buf.S284[i] = 42
	}

	buf.Clear()

	for _, v := range buf.S284 {
		if v != 0 {
			t.Errorf("expected 0, got %d", v)
		}
	}
}

func TestDecay(t *testing.T) {
	buf := GetBuffer(4)

	defer PutBuffer(buf)

	offL, offR := Fp284(256), Fp284(512)

	buf.Decay(&offL, &offR, 2)

	if offL >= 256 || offR >= 512 {
		t.Errorf("decay did not decrease offsets")
	}
}

func TestStereoFill(t *testing.T) {
	buf := GetBuffer(4)

	defer PutBuffer(buf)

	offL, offR := Fp284(256), Fp284(512)

	buf.StereoFill(&offL, &offR, 2)

	if buf.S284[0] == 0 && buf.S284[1] == 0 {
		t.Errorf("stereo fill did not fill values")
	}
}

func TestConvert284ToPCM(t *testing.T) {
	buf := GetBuffer(4)

	defer PutBuffer(buf)

	// Fill S284 with known values spanning negative, zero, positive, full-scale
	buf.S284[0] = Fp284(-1 << 20) // -1.0 nominal
	buf.S284[1] = Fp284(-0x10000) // small negative
	buf.S284[2] = Fp284(0)        // zero
	buf.S284[3] = Fp284(1 << 20)  // +1.0 nominal
	tests := []struct {
		name     string
		fn       func(*Buffer, []byte) int
		bytesPer int
	}{
		{"PCM8", (*Buffer).ToPCM8, 1},
		{"PCM16", (*Buffer).ToPCM16, 2},
		{"PCM24", (*Buffer).ToPCM24, 3},
		{"PCM32", (*Buffer).ToPCM32, 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := make([]byte, len(buf.S284)*tc.bytesPer)
			n := tc.fn(buf, out)

			if n != len(buf.S284) {
				t.Errorf("%s: expected %d samples, got %d", tc.name, len(buf.S284), n)
			}

			// Check each sample
			for i := range buf.S284 {
				var want []byte

				switch tc.name {
				case "PCM8":
					v := Fp284ToPCM8(buf.S284[i])
					want = []byte{byte(v)}
				case "PCM16":
					v := Fp284ToPCM16(buf.S284[i])
					want = []byte{byte(v), byte(v >> 8)}
				case "PCM24":
					v := Fp284ToPCM24(buf.S284[i])
					want = []byte{byte(v), byte(v >> 8), byte(v >> 16)}
				case "PCM32":
					v := Fp284ToPCM32(buf.S284[i])
					want = []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
				}

				got := out[i*tc.bytesPer : (i+1)*tc.bytesPer]

				if !bytes.Equal(got, want) {
					t.Errorf("%s: sample %d, got %v, want %v", tc.name, i, got, want)
				}
			}
		})
	}
}

func TestPCMTo284RoundTrip(t *testing.T) {
	buf := GetBuffer(4)

	defer PutBuffer(buf)

	// Representative Fp284 values: full negative, small negative, zero, full positive
	buf.S284[0] = -Ref284 // full negative
	buf.S284[1] = -65536  // small negative
	buf.S284[2] = 0       // zero
	buf.S284[3] = Ref284  // full positive
	tests := []struct {
		name     string
		toPCM    func(*Buffer, []byte) int
		fromPCM  func([]Fp284, []byte) int
		bytesPer int
		pcmBits  int
	}{
		{"PCM8", (*Buffer).ToPCM8, Convert8To284, 1, 8},
		{"PCM16", (*Buffer).ToPCM16, Convert16To284, 2, 16},
		{"PCM24", (*Buffer).ToPCM24, Convert24To284, 3, 24},
		{"PCM32", (*Buffer).ToPCM32, Convert32To284, 4, 32},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := make([]byte, len(buf.S284)*tc.bytesPer)
			// Convert Fp284 -> PCM
			n := tc.toPCM(buf, out)

			if n != len(buf.S284) {
				t.Errorf("%s: expected %d PCM samples, got %d", tc.name, len(buf.S284), n)
			}

			// Convert PCM -> Fp284
			round := make([]Fp284, len(buf.S284))
			m := tc.fromPCM(round, out)

			if m != len(buf.S284) {
				t.Errorf("%s: expected %d fp28.4 samples, got %d", tc.name, len(buf.S284), m)
			}

			// Compute 1 PCM LSB in Fp284 units
			// Fp284 has 4 fractional bits, PCM has pcmBits
			// Compute 1 PCM LSB in Fp284 units
			shift := max(28-tc.pcmBits, 0) + 4 // 28 bits integer + 4 bits fractional - pcmBits
			maxDiff := int32(1) << shift

			// Compare original vs round-trip
			for i := range buf.S284 {
				want := buf.S284[i]
				got := round[i]
				diff := abs(got - want)

				if diff > maxDiff {
					t.Errorf("%s: sample %d round-trip error too large, got %d, want %d (tolerance ±%d)", tc.name, i, got, want, maxDiff)
				}
			}
		})
	}
}

func abs(f Fp284) Fp284 {
	if f < 0 {
		return -f
	}

	return f
}

func TestInterleave(t *testing.T) {
	a := GetBuffer(2)
	b := GetBuffer(2)

	defer func() {
		PutBuffer(a)
		PutBuffer(b)
	}()

	a.S284[0], a.S284[1] = 1, 2
	b.S284[0], b.S284[1] = 3, 4
	out := a.Interleave(b)
	expected := []Fp284{1, 3, 2, 4}
	got := make([]byte, 4)

	for i, b := range out.All() {
		got[i] = byte(b)
	}

	if !bytes.Equal(got, []byte{1, 3, 2, 4}) {
		t.Errorf("interleave failed: got %v, want %v", out.S284, expected)
	}
}
