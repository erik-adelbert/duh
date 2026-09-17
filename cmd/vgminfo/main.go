// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// aacinfo displays information about AAC files, including their PCM format if requested.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	vgm "github.com/erik-adelbert/duh/pkg/vgmo3"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	input *os.File
	pcm   bool
}

func main() {
	a := getArgs()

	in := a.input
	defer in.Close() //nolint:errcheck // can't do anything if it fails

	infos, err := in.Stat()
	die(err, "Failed to get file info", 1)

	src, err := vgm.Decode(in, infos.Size())
	die(err, "Failed to create VGM decoder", 1)

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

	a.input, err = os.Open(*fs.path)
	die(err, "Failed to open AAC file", 1)

	return a
}

type flags struct {
	path *string
	pcm  *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Display information about AAC files\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path: flag.String("f", "", "path to AAC file"),
		pcm:  flag.Bool("pcm", false, "output PCM format and exit"),
	}

	flag.Parse()

	return fs
}

var die = xflags.MkDie(os.Stderr)
