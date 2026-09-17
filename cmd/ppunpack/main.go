// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// ppunpack decompresses files compressed with the PP20 algorithm.
package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/pp20"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	const ExitError = 1

	a := getArgs()

	var (
		in io.Reader

		err error
	)

	// Attempt to open the file with PP20 decompression first
	if a.inPath == "-" {
		// Read from stdin with a limit to prevent excessive memory usage
		verbosef(
			"Reading from stdin with a limit of %d bytes...\n",
			pp20.MaxCompressedSize+1,
		)

		// mmmcmp does not support streaming decompression,
		// so we need to read the entire input into memory first.
		limited := io.LimitReader(os.Stdin, pp20.MaxCompressedSize+1)

		packed, err := io.ReadAll(limited)
		die(err, "", ExitError)

		size := int64(len(packed))

		in, err = pp20.Unpack(bytes.NewReader(packed), size)
		die(err, "", ExitError)
	} else {
		f, err := os.Open(a.inPath)
		die(err, "Error opening input file", ExitError)
		defer f.Close() //nolint:errcheck // can't do anything if it fails

		infos, _ := f.Stat()

		in, err = pp20.Unpack(f, infos.Size())
		die(err, "", ExitError)
	}

	// Determine the output destination
	out := os.Stdout // Default to stdout

	if a.outPath != "-" {
		// An output file is specified, create it
		out, err := os.Create(a.outPath)
		die(err, "Error creating output file", ExitError)

		defer out.Close() //nolint:errcheck // can't do anything if it fails
	}

	// Copy the decompressed data to the output
	_, err = io.Copy(out, in)
	die(err, "Error writing to output file", ExitError)

	verbosef("Decompression completed successfully")
}

type flags struct {
	inPath, outPath *string
}

func parseFlags() flags {
	var f flags

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Decompress files compressed with the PP20 algorithm\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

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
