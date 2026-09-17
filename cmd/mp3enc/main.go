// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// mp3enc encodes raw PCM audio data into MP3 format.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/mp3"
	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	ifmt pcm.Format
	ofmt pcm.Format

	out io.WriteCloser
}

func main() {
	a := getArgs()

	var in io.Reader = os.Stdin

	if a.ifmt != a.ofmt {
		conv, err := pcm.NewConverter(a.ifmt, a.ofmt)
		die(err, "Failed to create PCM converter", 1)

		in = pcm.OpenConv(in, conv)
	}

	out, err := mp3.Encode(a.out, a.ofmt)
	die(err, "Failed to create MP3 writer", 1)

	_, err = io.Copy(out, in)
	die(err, "Failed to copy MP3 data", 1)

	err = a.out.Close()
	die(err, "Failed to close output", 1)
}

type flags struct {
	ifmt, ofmt *string
	out        *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Encode raw PCM audio data into MP3 format\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	f := flags{
		ifmt: flag.String("if", "", "input PCM format"),
		ofmt: flag.String("of", "", "output MP3 format"),
		out:  flag.String("o", "-", "output MP3 file (default: stdout)"),
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
		die(err, "Invalid number of arguments", 2)
	}

	return f
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

	a.ofmt, err = pcm.ParseFormat(*f.ofmt)
	die(err, "Invalid PCM output format", 2)

	if *f.out == "-" {
		a.out = writeNopCloser{os.Stdout}
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
