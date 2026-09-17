// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vgmdec decodes VGM files and writes the raw PCM data to stdout.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/erik-adelbert/duh/pkg/vgmo3"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	f := getArgs().file
	defer f.Close() // nolint:errcheck // can't do anything if it fails

	infos, err := f.Stat()
	die(err, "Failed to get file info", 1)

	in, err := vgmo3.Decode(f, infos.Size())
	die(err, "Failed to create VGM reader", 1)

	_, err = io.Copy(os.Stdout, in)
	die(err, "Failed to decode VGM file", 1)
}

type args struct {
	file *os.File
}

func getArgs() args {
	fs := parseFlags()

	file, err := os.Open(*fs.path)
	die(err, "Failed to open VGM file", 1)

	return args{
		file: file,
	}
}

type flags struct {
	path *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Decode VGM files to raw PCM data\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path: flag.String("f", "", "path to VGM file"),
	}

	flag.Parse()

	return fs
}

var die = xflags.MkDie(os.Stderr)
