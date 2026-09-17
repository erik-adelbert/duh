// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package wav provides WAV file reader and writer for LPCM audio data.
package wav

import (
	"os"
)

// Example demonstrates how to read from an input WAV file and write
// to an output WAV file.
func Example(in *os.File, out *os.File) (err error) {
	// Read WAV file and extract PCM samples
	src, _ := Decode(in)

	// Write PCM samples to the WAV file
	dst, _ := Encode(out, src.Format())
	defer func() {
		err = dst.Close() // capture any error from closing the writer
	}()

	// Read PCM samples from the input WAV file and process them as needed
	// For example, we read samples into a buffer:

	var n, nread, nwritten int

	buffer := make([]byte, 64*kB) // Adjust buffer size as needed
	for {
		// Read PCM samples from the input WAV file
		n, err = src.Read(buffer)
		nread += n

		if err != nil {
			break
		}

		// Process the buffer as needed

		// Write PCM samples to the output WAV file
		n, err = dst.Write(buffer)
		nwritten += n

		if err != nil {
			break
		}
	}

	_, _, _ = nread, nwritten, err // Use err, nread and nwritten as needed

	return
}
