// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pp20

import (
	"bytes"
	"io"
	"testing"
)

/*
This fuzz test targets the PP20 decompressor.
Goals:
	- Never panic
	- Never infinite loop
	- Never write out of bounds
	- Correct EOF behavior
	- Bitstream safety

This is especially important because PP20:
	- Reads bits backwards
	- Uses negative indexing logic
	- Copies overlapping regions (LZ-style)
*/

// ----------------------------
// End-to-end fuzzing
// ----------------------------
func FuzzOpenAndRead(f *testing.F) {
	// --- Seed corpus ---
	// Empty input
	f.Add([]byte{})
	// Small junk
	f.Add(make([]byte, 64))
	// Minimal invalid header
	f.Add([]byte("PP20\x00\x00\x00\x00"))
	// Minimal structurally-valid PP20-ish file
	f.Add(func() []byte {
		buf := make([]byte, 256)

		// Signature
		copy(buf[:4], []byte("PP20"))

		// Efficiency bytes (valid: 9..15)
		buf[4] = 9
		buf[5] = 9
		buf[6] = 9
		buf[7] = 9
		// Footer: uncompressed size + skip
		// size = 512 (minimum valid)
		buf[len(buf)-4] = 0x00
		buf[len(buf)-3] = 0x02
		buf[len(buf)-2] = 0x00
		buf[len(buf)-1] = 0x00 // skip = 0

		return buf
	}())
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 8 {
			return
		}

		r := bytes.NewReader(data)
		rd, err := Unpack(r, int64(len(data)))

		if err != nil {
			// Invalid input is expected
			return
		}

		// Sequential read
		buf := make([]byte, 1024)

		for {
			_, err := rd.Read(buf)

			if err == io.EOF {
				break
			}

			if err != nil {
				return
			}
		}

		// Random access read
		// _, _ = rd.ReadAt(buf, 0)
	})
}

// ----------------------------
// Header-focused fuzzing
// Forces valid PP20 signature
// ----------------------------
func FuzzHeaderOnly(f *testing.F) {
	f.Add(make([]byte, 256))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 256 {
			return
		}

		// Force PP20 signature
		copy(data[:4], []byte("PP20"))

		// Force valid efficiency bytes
		for i := 4; i < 8; i++ {
			data[i] = 9
		}

		// Footer: uncompressed size + skip
		data[len(data)-4] = 0x00
		data[len(data)-3] = 0x02
		data[len(data)-2] = 0x00
		data[len(data)-1] &= 0x1F // skip <= 31
		r := bytes.NewReader(data)
		rd, err := Unpack(r, int64(len(data)))

		if err != nil {
			return
		}

		_, _ = io.ReadAll(rd)
	})
}

// ----------------------------
// Bitstream stress fuzzing
// Targets readBits logic indirectly
// ----------------------------
func FuzzBitHeavy(f *testing.F) {
	f.Add(bytes.Repeat([]byte{0xFF}, 256))
	f.Add(bytes.Repeat([]byte{0x00}, 256))
	f.Add(bytes.Repeat([]byte{0xAA}, 256))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 256 {
			return
		}

		copy(data[:4], []byte("PP20"))

		for i := 4; i < 8; i++ {
			data[i] = 9
		}

		data[len(data)-4] = 0x00
		data[len(data)-3] = 0x02
		data[len(data)-2] = 0x00
		data[len(data)-1] = 0x00
		r := bytes.NewReader(data)
		rd, err := Unpack(r, int64(len(data)))

		if err != nil {
			return
		}

		// Read small chunks to exercise state
		buf := make([]byte, 7)

		for range 32 {
			_, err := rd.Read(buf)

			if err != nil {
				return
			}
		}
	})
}
