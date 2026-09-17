// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/erik-adelbert/duh/internal/tracker/loader/s3m"
	"github.com/erik-adelbert/duh/pkg/mmcmp"
)

func main() {
	s3mFile := flag.String("f", "", "Module file path (required)")
	flag.Parse()

	if *s3mFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	var in s3m.Reader

	f, err := os.Open(*s3mFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close() //nolint:errcheck // can't do anything if it fails

	infos, _ := f.Stat()

	// Attempt to open the file with MMCMP decompression first
	in, err = mmcmp.Unpack(f, infos.Size())

	switch err {
	case nil:
		fmt.Println("MMCMP decompressed...")
	default:
		// If MMCMP decompression fails, try to open the file normally
		in, err = os.Open(*s3mFile)

		if err != nil {
			fmt.Printf("Error opening input file: %v\n", err)
			os.Exit(1)
		}
	}

	mod, err := s3m.New(in)

	if err != nil {
		fmt.Printf("Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully read: %s\n", mod.String())
	fmt.Println(mod.Header)
}
