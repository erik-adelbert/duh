// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// pcmconv converts PCM data from one format to another.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

const DefaultBufSize = 64 * kB

type args struct {
	ifmt pcm.Format
	ofmt pcm.Format
}

func main() {
	a := getArgs()

	nconv := 0

	conv, err := pcm.NewConverter(a.ofmt, a.ifmt)
	die(err, "Failed to create PCM converter", 1)

	verbosef("Converting from %s to %s\n", a.ifmt, a.ofmt)

	isz := a.ifmt.Align(DefaultBufSize)
	isz = max(isz, int(a.ifmt.BufferSize(200*ms)))

	osz, _ := conv.Ratio(isz)
	ofsz := int(a.ofmt.FrameSize())

	ibuf := make([]byte, isz)
	obuf := make([]byte, osz+ofsz)

	var n int

	for {
		n, err = os.Stdin.Read(ibuf)

		if err != nil || n == 0 {
			break
		}

		n = conv.Process(obuf, ibuf[:n])
		nconv += n

		n, err = os.Stdout.Write(obuf[:n])

		if err != nil || n == 0 {
			break
		}
	}

	if err != nil && err != io.EOF {
		die(err, "Failed to convert PCM data", 1)
	}

	n = conv.Flush(obuf)

	_, err = os.Stdout.Write(obuf[:n])
	die(err, "Failed to write PCM data", 1)

	verbosef("Successfully converted %d bytes\n", nconv)
}

func getArgs() args {
	fs := parseFlags()

	ifmt, err := pcm.ParseFormat(*fs.ifmt)
	die(err, "Failed to parse input format", 2)

	ofmt, err := pcm.ParseFormat(*fs.ofmt)
	die(err, "Failed to parse output format", 2)

	return args{
		ifmt: ifmt,
		ofmt: ofmt,
	}
}

type flags struct {
	ifmt *string
	ofmt *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Convert PCM data from one format to another\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		ifmt: flag.String("if", "", "input format (s16c2r44100)"),
		ofmt: flag.String("of", "", "output format (s24c2r44100)"),
	}

	flag.Parse()

	return fs
}

var (
	die      = xflags.MkDie(os.Stderr)
	verbosef = xflags.MkVerbosef(os.Stderr)
)

const (
	kB = 1024
	ms = time.Millisecond
)
