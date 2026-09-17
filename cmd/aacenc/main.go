// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// aacenc encodes raw PCM audio data into AAC format.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/tphakala/aac"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	ifmt pcm.Format
	ofmt aac.Format

	out io.WriteCloser
}

func main() {
	a := getArgs()

	in := io.NopCloser(os.Stdin)

	nchan := a.ifmt.ChannelCount()
	sr := a.ifmt.SampleRate()

	cvfmt, _ := pcm.NewFormat(pcm.SignedIntLE, 16, nchan, sr)

	conv, err := pcm.NewConverter(a.ifmt, cvfmt)
	die(err, "Failed to create PCM converter", 1)

	in = io.NopCloser(pcm.OpenConv(in, conv))

	out, err := aac.Encode(a.out, a.ofmt)
	die(err, "Failed to create AAC writer", 1)

	_, err = io.Copy(out, in)
	die(err, "Failed to copy AAC data", 1)

	err = out.Close()
	die(err, "Failed to close AAC writer", 1)

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
		fmt.Fprintf(os.Stderr, "Encode raw PCM audio data into AAC format\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	f := flags{
		ifmt: flag.String("if", "", "input PCM format"),
		ofmt: flag.String("of", "", "output AAC format"),
		out:  flag.String("o", "-", "output AAC file (default: stdout)"),
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

	a.ofmt, err = aac.ParseFormat(*f.ofmt)
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
