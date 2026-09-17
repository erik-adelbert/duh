// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// unmmcmp decompresses files compressed with the MMCMP algorithm.
package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/mmcmp"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	const ExitError = 1

	a := getArgs()

	var (
		in  io.Reader
		err error
	)

	// Open the input source
	if a.inPath == "-" {
		// Read from stdin with a limit to prevent excessive memory usage
		verbosef(
			"Reading from stdin with a limit of %d bytes...\n",
			mmcmp.MaxCompressedSize+1,
		)

		// mmmcmp does not support streaming decompression,
		// so we need to read the entire input into memory first.
		limited := io.LimitReader(os.Stdin, mmcmp.MaxCompressedSize+1)

		packed, err := io.ReadAll(limited)
		die(err, "Failed to read input from stdin", ExitError)

		size := int64(len(packed))

		in, err = mmcmp.Unpack(bytes.NewReader(packed), size)
		die(err, "Failed to create mmcmp reader", ExitError)

	} else {
		f, err := os.Open(a.inPath)
		die(err, "Failed to open input file", ExitError)
		defer f.Close() //nolint:errcheck // can't do anything if it fails

		infos, err := f.Stat()
		die(err, "Failed to stat input file", ExitError)

		in, err = mmcmp.Unpack(f, infos.Size())
		die(err, "Failed to open input file", ExitError)
	}

	// Ready the output destination
	out := os.Stdout // Default to stdout

	if a.outPath != "-" {
		// An output file is specified, create it
		out, err := os.Create(a.outPath)
		die(err, "", ExitError)

		defer out.Close() //nolint:errcheck // can't do anything if it fails
	}

	// Copy the decompressed data to the output destination
	_, err = io.Copy(out, in)
	die(err, "", ExitError)

	verbosef("Decompression completed successfully")
}

// CLI arguments and flags

type flags struct {
	inPath, outPath *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Decompress files compressed with the MMCMP algorithm\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	var f flags

	f.inPath = flag.String("i", "", "Input file path")
	f.outPath = flag.String("o", "-", "Output file path") // Default to stdout

	flag.Parse()

	return f
}

type args struct {
	inPath, outPath string
}

func getArgs() args {
	f := parseFlags()

	if *f.inPath == "" {
		// If no input file is provided, default to reading from stdin
		*f.inPath = "-"

		// Unless an input file is specified on the command line
		if len(flag.Args()) == 1 {
			*f.inPath = flag.Args()[0]
		}
	}

	return args{
		inPath:  *f.inPath,
		outPath: *f.outPath,
	}
}

// CLI helper functions

var (
	die      = xflags.MkDie(os.Stderr)
	verbosef = xflags.MkVerbosef(os.Stderr)
)
