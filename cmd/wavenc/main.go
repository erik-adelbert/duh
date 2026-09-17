// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// wavenc encodes raw PCM audio data into WAV format.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/wav"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	a := getArgs()

	var (
		out io.WriteCloser
		err error
	)

	in := io.NopCloser(os.Stdin)

	if a.ifmt != a.ofmt {
		conv, err := pcm.NewConverter(a.ifmt, a.ofmt)
		die(err, "Failed to create PCM converter", 1)

		in = io.NopCloser(pcm.OpenConv(in, conv))
	}

	if a.bufout {
		// buffered path from stdout: write to a temporary file first
		const maxBufferedSize = 1 * GB

		in = io.NopCloser(io.LimitReader(in, maxBufferedSize))

		tmp, err := os.CreateTemp("", "*.wav")
		die(err, "Failed to create temporary output file", 1)

		defer func() { _ = os.Remove(tmp.Name()) }()

		w, err := wav.Encode(tmp, a.ofmt)
		die(err, "Failed to create WAV writer", 1)

		_, err = io.Copy(w, in)
		die(err, "Failed to write WAV data", 1)

		err = w.Close()
		die(err, "Failed to finalize WAV file", 1)

		err = tmp.Close()
		die(err, "Failed to close temporary WAV file", 1)

		in, err = os.Open(tmp.Name())
		die(err, "Failed to reopen temporary WAV file", 1)

		out = a.out
	} else {
		// direct path: write directly to the output file
		out, err = wav.Encode(a.out, a.ofmt)
		die(err, "Failed to create WAV writer", 1)
	}

	_, err = io.Copy(out, in)
	die(err, "Failed to copy WAV data", 1)

	if !a.bufout {
		err = out.Close()
		die(err, "Failed to finalize WAV file", 1)
	}

	err = a.out.Close()
	die(err, "Failed to close output", 1)

	err = in.Close()
	die(err, "Failed to close input", 1)
}

type flags struct {
	ifmt, ofmt *string
	out        *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Encode raw PCM audio data into WAV format\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	f := flags{
		ifmt: flag.String("if", "", "input PCM format"),
		ofmt: flag.String("of", "", "output WAV format"),
		out:  flag.String("o", "-", "output WAV file (default: stdout)"),
	}

	flag.Parse()

	switch len(flag.Args()) {
	case 0:
		// no positional arguments provided, do nothing
	case 1:
		if *f.out != "-" {
			err := errors.New("output file already specified")
			die(err, "Invalid output file argument", 2)
		}

		*f.out = flag.Args()[0]
	default:
		err := errors.New("too many arguments")
		die(err, "Failed to parse arguments", 2)
	}

	return f
}

type args struct {
	ifmt pcm.Format
	ofmt wav.Format

	out    io.WriteCloser
	bufout bool
}

func getArgs() args {
	var (
		a   args
		err error
	)

	f := parseFlags()

	a.ifmt, err = pcm.ParseFormat(*f.ifmt)
	die(err, "Invalid PCM input format", 2)

	if *f.ofmt == "" {
		*f.ofmt = *f.ifmt
	}

	a.ofmt, err = wav.ParseFormat(*f.ofmt)
	die(err, "Invalid WAV output format", 2)

	if *f.out == "-" {
		a.out = writeNopCloser{os.Stdout}
		a.bufout = true
	} else {
		a.out, err = os.OpenFile(*f.out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		die(err, "Failed to create output file", 1)
	}

	return a
}

var die = xflags.MkDie(os.Stderr)

const GB = 1 << 30

type writeNopCloser struct {
	io.Writer
}

func (w writeNopCloser) Close() error {
	return nil
}
