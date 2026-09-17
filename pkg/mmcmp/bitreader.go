// Copyright (c) 2024 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

type bitReader struct {
	buf []byte
	idx int

	bitCnt int
	bitBuf uint64
}

// newBitReader creates a new bitReader for the given byte slice.
func newBitReader(buf []byte) *bitReader {
	return new(bitReader{buf: buf})
}

// readN reads n bits from the bitReader and returns them as an integer.
func (br *bitReader) readN(n int) (int, error) {
	// Validate bit count
	switch {
	case n == 0:
		return 0, nil // Zero bits requested, return 0
	case n < 0:
		return 0, mkError(errBits, "negative bit count")
	case n > 63:
		return 0, mkError(errBits, "bit count too large")
	}

	// Fill the bit buffer until it has at least n bits
	for br.bitCnt < n && br.idx < len(br.buf) {
		b := br.buf[br.idx]

		br.bitBuf |= uint64(b) << br.bitCnt
		br.bitCnt += 8

		br.idx++
	}

	// If there are not enough bits left, inject 0s
	br.bitCnt += ((n - br.bitCnt + 7) / 8) * 8 // align to next byte boundary

	// Extract the requested bits from the bit buffer
	mask := (uint64(1) << n) - 1

	bits := int(br.bitBuf & mask)
	br.bitBuf >>= n

	// Consume the bits
	br.bitCnt -= n

	return bits, nil
}
