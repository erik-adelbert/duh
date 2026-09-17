// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package mp3 provides functionality for reading and processing MP3 audio files.
package mp3

import "io"

// Example demonstrates how to read MP3 data from an input reader,
// process it as PCM, and write it to an output writer as MP3.
func Example(out io.Writer, in io.Reader) {
	r, _ := Decode(in) // ignoring error for example purposes

	w, _ := Encode(out, r.Format())
	// ignoring error for example purposes

	buf := make([]byte, 4096) // Adjust buffer size as needed

	for {
		n, err := r.Read(buf) // Read and decode MP3 data to PCM

		if err != nil {
			break
		}

		// Process the PCM data here if needed

		_, err = w.Write(buf[:n]) // Encode and write PCM data to MP3

		if err != nil {
			break
		}
	}
}
