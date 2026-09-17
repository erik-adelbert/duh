// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audio

import (
	"iter"

	"github.com/erik-adelbert/duh/internal/help"
)

const (
	// Fixed-point 28.4 minimum value
	Min284 = -(1 << 27)
	// Fixed-point 28.4 maximum value
	Max284 = (1 << 27) - 1
	// Fixed-point 28.4 reference value (0 dBFS)
	log284       = 20
	Ref284 int32 = 1 << log284
)

// Int284 returns the integer part of an fp28.4 sample.
func Int284(f Fp284) int32 {
	return f >> 4
}

// Frac284 returns the fractional part of an fp28.4 sample.
func Frac284(f Fp284) int32 {
	return f & 0xF
}

// Convert fp28.4 sample to 8-bit PCM sample.
func Fp284ToPCM8(f Fp284) PCM8 {
	f = help.Clamp(f, Min284, Max284)
	x := help.Clamp(pcmScale(f, 8), -128, 127)

	return PCM8(x + 128)
}

func PCM8ToFp284(p PCM8) Fp284 {
	return pcmUnscale(int32(p)-128, 8)
}

func Fp284ToPCM16(f Fp284) PCM16 {
	f = help.Clamp(f, Min284, Max284)

	return PCM16(pcmScale(f, 16))
}

func PCM16ToFp284(p PCM16) Fp284 {
	return pcmUnscale(int32(p), 16)
}

func Fp284ToPCM24(f Fp284) PCM24 {
	f = help.Clamp(f, Min284, Max284)

	return PCM24(pcmScale(f, 24))
}

func PCM24ToFp284(p PCM24) Fp284 {
	return pcmUnscale(int32(p), 24)
}

func Fp284ToPCM32(f Fp284) PCM32 {
	f = help.Clamp(f, Min284, Max284)

	return PCM32(pcmScale(f, 32))
}

func PCM32ToFp284(p PCM32) Fp284 {
	return pcmUnscale(int32(p), 32)
}

// Convert284To8 converts fp28.4 samples in src to 8-bit PCM samples in dst.
func Convert284To8(dst []byte, src []Fp284) int {
	n := min(len(dst), len(src))

	for i := range n {
		p8 := Fp284ToPCM8(src[i])
		dst[i] = byte(p8)
	}

	return n
}

// Convert284To16 converts fp28.4 samples in src to 16-bit PCM samples in dst.
func Convert284To16(dst []byte, src []Fp284) int {
	n := min(len(dst)/2, len(src))

	for i := range n {
		p16 := Fp284ToPCM16(src[i])
		dst[i*2+0] = byte(p16)
		dst[i*2+1] = byte(p16 >> 8)
	}

	return n
}

// Convert284To32 converts fp28.4 samples in src to 24-bit PCM samples in dst.
func Convert284To24(dst []byte, src []Fp284) int {
	n := min(len(dst)/3, len(src))

	for i := range n {
		p24 := Fp284ToPCM24(src[i])
		dst[i*3+0] = byte(p24)
		dst[i*3+1] = byte(p24 >> 8)
		dst[i*3+2] = byte(p24 >> 16)
	}

	return n
}

// Convert284To32 converts fp28.4 samples in src to 32-bit PCM samples in dst.
func Convert284To32(dst []byte, src []Fp284) int {
	n := min(len(dst)/4, len(src))

	for i := range n {
		p32 := Fp284ToPCM32(src[i])
		dst[i*4+0] = byte(p32)
		dst[i*4+1] = byte(p32 >> 8)
		dst[i*4+2] = byte(p32 >> 16)
		dst[i*4+3] = byte(p32 >> 24)
	}

	return n
}

// Convert8To284 converts 8-bit PCM samples in src to fp28.4 format in dst.
func Convert8To284(dst []Fp284, src []byte) int {
	n := min(len(dst), len(src))

	for i := range n {
		dst[i] = PCM8ToFp284(PCM8(src[i]))
	}

	return n
}

// Convert16To284 converts 16-bit PCM samples in src to fp28.4 format in dst.
func Convert16To284(dst []Fp284, src []byte) int {
	n := min(len(dst), len(src)/2)

	for i := range n {
		p16 := PCM16(src[i*2]) | PCM16(src[i*2+1])<<8
		dst[i] = PCM16ToFp284(p16)
	}

	return n
}

// Convert24To284 converts 24-bit PCM samples in src to fp28.4 format in dst.
func Convert24To284(dst []Fp284, src []byte) int {
	n := min(len(dst), len(src)/3)

	for i := range n {
		p24 := PCM24(src[i*3]) | PCM24(src[i*3+1])<<8 | PCM24(src[i*3+2])<<16
		p24 = (p24 << 8) >> 8 // sign-extend 24-bit
		dst[i] = PCM24ToFp284(p24)
	}

	return n
}

// Convert32To284 converts 32-bit PCM samples in src to fp28.4 format in dst.
func Convert32To284(dst []Fp284, src []byte) int {
	n := min(len(dst), len(src)/4)

	for i := range n {
		p32 := PCM32(src[i*4]) | PCM32(src[i*4+1])<<8 | PCM32(src[i*4+2])<<16 | PCM32(src[i*4+3])<<24
		dst[i] = PCM32ToFp284(p32)
	}

	return n
}

// AllPCM8 returns an iterator that yields 8-bit PCM samples converted from fp28.4 samples.
func AllPCM8(fps iter.Seq2[int, Fp284]) iter.Seq2[int, PCM8] {
	return func(yield func(int, PCM8) bool) {
		for i, f := range fps {
			if !yield(i, Fp284ToPCM8(f)) {
				return
			}
		}
	}
}

// AllPCM16 returns an iterator that yields 16-bit PCM samples converted from fp28.4 samples.
func AllPCM16(fps iter.Seq2[int, Fp284]) iter.Seq2[int, PCM16] {
	return func(yield func(int, PCM16) bool) {
		for i, f := range fps {
			if !yield(i, Fp284ToPCM16(f)) {
				return
			}
		}
	}
}

// AllPCM24 returns an iterator that yields 24-bit PCM samples converted from fp28.4 samples.
func AllPCM24(fps iter.Seq2[int, Fp284]) iter.Seq2[int, PCM24] {
	return func(yield func(int, PCM24) bool) {
		for i, f := range fps {
			if !yield(i, Fp284ToPCM24(f)) { // Convert fp28.4 to 24-bit PCM

				return
			}
		}
	}
}

// AllPCM32 returns an iterator that yields 32-bit PCM samples converted from fp
func AllPCM32(fps iter.Seq2[int, Fp284]) iter.Seq2[int, PCM32] {
	return func(yield func(int, PCM32) bool) {
		for i, f := range fps {
			if !yield(i, Fp284ToPCM32(f)) { // Convert fp28.4 to 32-bit PCM

				return
			}
		}
	}
}

// pcmShift returns the bit shift needed to map Ref284 to full-scale PCM.
// Assumes Ref284 is a power of two (e.g. 1<<20).
func pcmShift(pcmBits int) int {
	// msb := bits.Len32(uint32(Ref284)) - 1
	msb := log284

	return msb - (pcmBits - 1)
}

// clampPCM clamps an int64 value to the valid range for the given PCM bit depth.
func clampPCM(x int64, pcmBits int) int32 {
	max := int64((1 << (pcmBits - 1)) - 1)
	min := -int64(1 << (pcmBits - 1))

	switch {
	case x > max:
		return int32(max)
	case x < min:
		return int32(min)
	}

	return int32(x)
}

// roundShift performs a right shift with rounding for int64 values.
func roundShift(x int64, shift uint) int64 {
	if x >= 0 {
		return (x + (1 << (shift - 1))) >> shift
	}

	return (x - (1 << (shift - 1))) >> shift
}

// pcmScale scales an fp28.4 sample to the given PCM bit depth.
func pcmScale(f Fp284, pcmBits int) int32 {
	shift := pcmShift(pcmBits)
	x := int64(Int284(f))

	switch {
	case shift < 0:
		x <<= -shift
	case shift > 0:
		x = roundShift(x, uint(shift))
	}

	return clampPCM(x, pcmBits)
}

// pcmUnscale converts a PCM sample to fp28.4 format.
func pcmUnscale(p int32, pcmBits int) Fp284 {
	const fracBits = 4

	shift := pcmShift(pcmBits)
	x := int64(p)

	switch {
	case shift < 0:
		x = roundShift(x, uint(-shift))
	case shift > 0:
		x <<= shift
	}

	return Fp284(x) << fracBits
}
