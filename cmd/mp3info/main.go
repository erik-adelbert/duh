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

	"github.com/erik-adelbert/duh/pkg/mp3"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	input io.ReadCloser
	pcm   bool
}

func main() {
	a := getArgs()

	in := a.input
	defer in.Close() //nolint:errcheck // can't do anything if it fails

	src, err := mp3.Decode(in)
	die(err, "Failed to create MP3 decoder", 1)

	if a.pcm {
		fmt.Println(src.Format())
		return
	}

	fmt.Println(src)
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

	if *fs.path == "-" {
		a.input = io.NopCloser(os.Stdin)
	} else {
		a.input, err = os.Open(*fs.path)
		die(err, "Failed to open MP3 file", 1)
	}

	return a
}

type flags struct {
	path *string
	pcm  *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Display information about MP3 files\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path: flag.String("f", "", "path to MP3 file"),
		pcm:  flag.Bool("pcm", false, "output PCM format and exit"),
	}

	flag.Parse()

	return fs
}

var die = xflags.MkDie(os.Stderr)
