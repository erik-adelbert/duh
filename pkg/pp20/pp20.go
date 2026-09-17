// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pp20 implements a reader for Amiga PowerPacker 2.0 compressed files.
// PP20 is a custom LZ77-style lossless data compression algorithm,
// originally used on the Amiga. It uses a bitwise data format combining
// literal runs and back-references (matches) to previously decompressed data.
package pp20

import (
	"io"
)

// Example demonstrates how to use the PP20 reader to decompress a file.
func Example(out io.Writer, packed io.ReaderAt, size int64) {
	// open a PP20 file for decompression
	unpacked, _ := Unpack(packed, size)

	// unpack the PP20 file and write the decompressed data to the output file
	_, _ = io.Copy(out, unpacked)
}
