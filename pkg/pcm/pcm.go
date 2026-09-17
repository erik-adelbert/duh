// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

// Package pcm provides PCM audio conversion compatible with the 9front/plan9 pcm library.
//
// Supported conversions include:
//
//   - signed and unsigned PCM
//   - little- and big-endian sample formats
//   - floating-point PCM
//   - A-law and μ-law
//   - sample-rate conversion
//   - channel count conversion
//
// Original implementation:
// https://git.9front.org/plan9front/9front/8b0705c14943f4f8b3aa12b5c194e32cc07d0936/sys/src/libpcm
package pcm

import (
	"fmt"
	"io"
	"log"
	"os"
)

func Example() {
	ifmt, _ := ParseFormat("s16c2r44100")
	ofmt, _ := ParseFormat(SLE16Stereo48k)

	// Create a new PCM converter from the input format to the output format.
	conv, err := NewConverter(ofmt, ifmt)

	if err != nil {
		log.Fatal(err)
	}

	// Create a new PCM writer that applies the conversion.
	w := NewConvWriter(os.Stdout, conv)

	// Convert
	if _, err := io.Copy(w, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}

	// Handle closing the PCM writer.
	if err := w.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}
