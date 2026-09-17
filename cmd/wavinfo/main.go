// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// mp3info displays information about MP3 files, including their PCM format if requested.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/wav"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	a := getArgs()

	src := a.input
	defer src.Close() //nolint:errcheck // can't do anything if it fails

	in, err := wav.Decode(src)
	die(err, "Failed to create WAV reader", 1)

	if a.pcm {
		fmt.Println(in.Format())
		return
	}

	fmt.Println(in)
}

type args struct {
	input io.ReadCloser
	name  string
	pcm   bool
}

func getArgs() args {
	fs := parseFlags()

	var (
		a   args
		err error
	)

	a.pcm = *fs.pcm

	switch len(flag.Args()) {
	case 0:
		if *fs.path == "" {
			err := errors.New("no input file specified")
			die(err, "Invalid input file argument", 2)
		}
	case 1:
		if *fs.path != "" {
			err := errors.New("input file already specified")
			die(err, "Invalid input file argument", 2)
		}
		*fs.path = flag.Args()[0]
	default:
		err := errors.New("too many positional arguments")
		die(err, "Failed to parse arguments", 2)
	}

	if fs.path == nil || *fs.path == "-" {
		a.input = io.NopCloser(os.Stdin)
	} else {
		a.input, err = os.Open(*fs.path)

		if err != nil {
			die(err, "Failed to open WAV file", 1)
		}

		a.name = *fs.path
	}

	return a
}

type flags struct {
	path *string
	pcm  *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Display information about WAV files\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path: flag.String("f", "", "path to WAV file"),
		pcm:  flag.Bool("pcm", false, "output PCM format and exit"),
	}

	flag.Parse()

	return fs
}

var die = xflags.MkDie(os.Stderr)
