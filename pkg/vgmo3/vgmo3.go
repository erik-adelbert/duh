// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vgmo3 provides functionality for reading and processing OPL3-only VGM (Video Game Music) files.
// The main use case is to read VGM files and stream the commands to an adhoc OPL3 device for playback.
// The package is designed to be used in conjunction with duh's opl3 package, which provides an
// implementation of the OPL3 sound chip.
package vgmo3

import (
	"fmt"
	"os"
)

func Example(path string, dump bool) (err error) {
	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	infos, err := src.Stat()
	if err != nil {
		return err
	}

	// Open the VGM file for reading
	in, err := Decode(src, infos.Size())
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	if dump {
		fmt.Print(in.DumpCommands())
		return
	}

	// read and process the PCM data as needed
	processPCM(in)

	return
}

func processPCM(_ *Decoder) {}
