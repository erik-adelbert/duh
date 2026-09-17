// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mmcmp implements a reader for Amiga MusicModuleCoMPress compressed files.
// MMCMP is a lossless compression format used in various audio files. It was
// originally developed by Emmanuel Giasson (Zirconia) in the late 1990s.
// It primarily uses a combination of Huffman coding and delta coding.
package mmcmp

import (
	"io"
)

// Example demonstrates how to use the MMCMP reader to decompress data
// from an input reader and write it to an output writer.
func Example(out io.Writer, packed io.ReaderAt, size int64) {
	unpacked, _ := Unpack(packed, size) // Create a new MMCMP reader with the input size

	// Unpack the MMCMP input and output the decompressed data
	_, _ = io.Copy(out, unpacked)
}
