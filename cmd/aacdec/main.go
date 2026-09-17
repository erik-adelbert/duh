// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// aacdec decodes AAC files and writes the raw PCM data to stdout.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/tphakala/aac"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

type args struct {
	input io.ReadCloser
}

func main() {
	a := getArgs()

	in := a.input
	defer in.Close() //nolint:errcheck // can't do anything if it fails

	src, err := aac.Decode(in)
	die(err, "Failed to create AAC reader", 1)

	_, err = io.Copy(os.Stdout, src)
	die(err, "Failed to decode AAC file", 1)
}

func getArgs() args {
	fs := parseFlags()

	switch len(flag.Args()) {
	case 0:
		// no positional arguments, do nothing
	case 1:
		if *fs.path != "-" {
			err := errors.New("already specified")
			die(err, "Invalid input file argument", 2)
		}

		*fs.path = flag.Args()[0]
	default:
		err := fmt.Errorf("too many positional arguments")
		die(err, "Invalid input file argument", 2)
	}

	if *fs.path == "-" {
		return args{
			input: io.NopCloser(os.Stdin),
		}
	}

	file, err := os.Open(*fs.path)
	die(err, "Failed to open file", 1)

	return args{
		input: file,
	}
}

type flags struct {
	path *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Decode AAC files to raw PCM data\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path: flag.String("f", "-", "path to AAC file (default: stdin)"),
	}

	flag.Parse()

	return fs
}

var die = xflags.MkDie(os.Stderr)
