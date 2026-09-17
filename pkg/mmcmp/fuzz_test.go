// Copyright (c) 2024 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

import (
	"bytes"
	"io"
	"testing"
)

/*
This file uses Go native fuzzing (Go 1.18+).
It fuzzes:
- Header parsing
- Block tables
- Decompression logic
- Bit reader
- Read / ReadAt behavior
Primary invariant:
- Never panic
- Never infinite loop
- Never write out of bounds
*/

// ----------------------------
// Full pipeline fuzzing
// ----------------------------
func FuzzOpenAndRead(f *testing.F) {
	// --- Seed corpus ---
	// Empty
	f.Add([]byte{})
	// Too small to be valid
	f.Add(make([]byte, 32))
	// Minimal valid-ish MMCP header seed
	f.Add([]byte{
		// fileHeader
		'z', 'i', 'R', 'C', 'O', 'N', 'i', 'a', // signature
		0x0E, 0x00, // header length = 14
		// blockHeader (14 bytes)
		0x01, 0x00, // version
		0x01, 0x00, // blkCnt
		0x20, 0x00, 0x00, 0x00, // uncompressed size
		0x18, 0x00, 0x00, 0x00, // block table offset
		0x00, // globCmp
		0x00, // fmtCmp
		// padding
		0x00, 0x00, 0x00, 0x00,
	})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}

		r := bytes.NewReader(data)
		rd, err := Unpack(r, int64(len(data)))

		if err != nil {
			// Invalid inputs are expected
			return
		}

		// Sequential read
		buf := make([]byte, 1024)

		for {
			n, err := rd.Read(buf)

			if err == io.EOF {
				break
			}

			if err != nil {
				return
			}

			if n == 0 {
				t.Fatal("Reader returned (0, nil)")
			}
		}

		// Random access read
		// for _, off := range []int64{0, 1, 2, 16, int64(len(data))} {
		// 	_, _ = rd.ReadAt(buf, off)
		// }
	})
}

// ----------------------------
// Structured header fuzzing
// Forces valid signature + header length
// ----------------------------
func FuzzValidHeader(f *testing.F) {
	f.Add(make([]byte, 256))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 24 {
			return
		}

		// Force valid file header
		copy(data[:8], []byte("ziRCONia"))

		data[8] = 0x0E
		data[9] = 0x00
		r := bytes.NewReader(data)
		rd, err := Unpack(r, int64(len(data)))

		if err != nil {
			return
		}

		_, _ = io.ReadAll(rd)
	})
}

// ----------------------------
// Bit reader fuzzing
// ----------------------------
func FuzzBitReader(f *testing.F) {
	f.Add([]byte{0x00})
	f.Add([]byte{0xFF})
	f.Add([]byte{0xAA, 0x55})
	f.Add([]byte{0x00, 0x00, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		br := newBitReader(data)

		// Try many read widths
		for i := range 1024 {
			n := (i % 17) + 1 // 1..17 bits
			_, err := br.readN(n)

			if err != nil {
				return
			}
		}
	})
}
