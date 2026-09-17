package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"

	"github.com/erik-adelbert/duh/internal/tracker/loader/mod"
	"github.com/erik-adelbert/duh/pkg/pp20"
)

func main() {
	modFile := flag.String("f", "", "Module file path (required)")

	flag.Parse()

	if *modFile == "" {
		flag.Usage()
		os.Exit(2)
	}

	var in mod.Reader

	f, err := os.Open(*modFile)
	if err != nil {
		fmt.Printf("Error opening input file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close() //nolint:errcheck // can't do anything if it fails

	infos, err := f.Stat()
	if err != nil {
		fmt.Printf("Error getting file info: %v\n", err)
		os.Exit(1)
	}

	// Attempt to open the file with MMCMP decompression first
	in, err = pp20.Unpack(f, infos.Size())

	switch err {
	case nil:
		fmt.Println("PP20 decompressed...")
	default:
		// If PP20 decompression fails, try to open the file normally
		in, err = os.Open(*modFile)

		if err != nil {
			fmt.Printf("Error opening input file: %v\n", err)
			os.Exit(1)
		}
	}

	m, err := mod.New(in)

	if err != nil {
		fmt.Printf("Error reading: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully read: %s\n", m.String())
	fmt.Println(m.Header)
}
