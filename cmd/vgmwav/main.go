// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vgmwav converts VGM files to WAV format.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	vgm "github.com/erik-adelbert/duh/pkg/vgmo3"
	"github.com/erik-adelbert/duh/pkg/wav"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	const ExitError = 1

	a := getArgs()

	// Open input file
	in, err := os.Open(a.infile)
	die(err, "Failed to open input file", ExitError)

	defer in.Close() //nolint:errcheck // can't do anything if it fails

	infos, err := in.Stat()
	die(err, "Failed to stat input file", ExitError)

	// Create a VGM reader
	src, err := vgm.Decode(in, infos.Size())
	die(err, "Failed to create VGM reader", ExitError)

	src.SetLoopCount(a.nloop)

	// Create output file, default to stdout
	out := os.Stdout

	if a.outfile != "-" {
		out, err = os.Create(a.outfile)
		die(err, "Failed to create output file", ExitError)

		defer out.Close() //nolint:errcheck // can't do anything if it fails
	}

	// Create a WAV writer for the output file
	duration := src.Duration().Truncate(time.Second)
	verbosef("Synthesizing %s WAV output to %s with format %s...\n", duration, a.outfile, a.outfmt)

	wave, err := wav.Encode(out, a.outfmt)
	die(err, "Failed to create WAV writer", ExitError)

	// Copy and convert the WAV data
	started := time.Now()

	_, err = io.Copy(wave, src)
	die(err, "Failed to synthesize VGM data", ExitError)

	err = wave.Close()
	die(err, "Failed to finalize WAV output", ExitError)

	verbosef("Finished in %s\n", time.Since(started).Truncate(time.Millisecond))
}

const (
	DefaultOutputFormat = "s16c2r44100"
	DefaultOutputFile   = "out.wav"
	MinLoopCount        = 1
)

type flags struct {
	infile, outfile, outfmt *string
	nloop                   *int
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Convert VGM files to WAV format\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		nloop: flag.Int("n", MinLoopCount, "number of loops to play (min/default 1)"),

		infile: flag.String("i", "", "input VGM file"),

		outfile: flag.String("o", DefaultOutputFile, "output WAV file"),
		outfmt:  flag.String("of", DefaultOutputFormat, "output format"),
	}

	flag.Parse()

	return fs
}

type args struct {
	infile, outfile string
	outfmt          wav.Format
	nloop           int
}

func getArgs() args {
	fs := parseFlags()

	a := fs.flatten()

	// Determine the input source
	if a.infile == "" {
		a.infile = "-" // Default to stdin

		pargs := flag.Args() // Positional arguments after flags

		if len(pargs) > 0 {
			a.infile = pargs[0]
		}
	}

	// Determine the output destination
	if a.outfile == "" {
		a.outfile = DefaultOutputFile
	}

	return a
}

func (a *flags) flatten() args {
	outfmt, err := wav.ParseFormat(*a.outfmt)

	if err != nil {
		outfmt, _ = wav.ParseFormat(DefaultOutputFormat)
	}

	return args{
		nloop: min(MinLoopCount, *a.nloop),

		infile: *a.infile,

		outfile: *a.outfile,
		outfmt:  outfmt,
	}
}

var (
	die      = xflags.MkDie(os.Stderr)
	verbosef = xflags.MkVerbosef(os.Stderr)
)
