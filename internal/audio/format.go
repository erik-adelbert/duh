// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

type PCM8 byte

func AsPCM8(i int8) PCM8 {
	return PCM8(byte(i) + 0x80)
}

func (p PCM8) AsInt8() int8 {
	return int8(int16(p) - 0x80)
}

func (p PCM8) AsByte() byte {
	return byte(p)
}

type PCM16 = int16
type PCM24 = int32
type PCM32 = int32

// Fp284 represents a fixed-point 28.4 audio sample.
type Fp284 = int32
