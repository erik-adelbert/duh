// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"

	"github.com/erik-adelbert/duh/pkg/xflags"
)

func main() {
	a := getArgs()

	if a.download {
		DownloadFMBanks()
	}

	var cfg Config

	if a.ini != "" {
		err := convert(a.ini, a.config)

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error converting ini to config: %v\n", err)
			os.Exit(1)
		}
	}

	if a.config == "" || a.output == "" {
		if a.download {
			return
		}

		flag.Usage()
		os.Exit(2)
	}

	err := cfg.Open(a.config)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening config: %v\n", err)
		os.Exit(1)
	}

	timer, err := NewKeyTimer(a.cache)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating key timer: %v\n", err)
		os.Exit(1)
	}
	timer.forceTimings = a.forceTimings
	defer timer.Close()

	ctrlc, stop := sighandle(os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctrlc

		rc := 0

		err = timer.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error dumping DB: %v\n", err)
			rc = 1
		}

		os.Exit(rc)
	}()

	db := cfg.DB()

	nbank, nmidi := len(db.Banks), len(db.Midis)
	nins, nop := len(db.Ins), len(db.Ops)
	fmt.Printf("Banks: %d, Midis: %d, Ins: %d, Ops: %d\n", nbank, nmidi, nins, nop)

	timer.TimeDB(db)

	fmt.Println("KeyTimer:", "hits:", timer.stats.nhit, "misses:", timer.stats.nmiss)

	err = db.Dump(a.output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error dumping DB: %v\n", err)
		os.Exit(1)
	}
}

type args struct {
	ini    string
	cache  string
	config string
	output string

	download     bool
	forceTimings bool
}

func getArgs() args {
	return parseFlags().flatten()
}

type flags struct {
	ini    *string
	cache  *string
	config *string
	output *string

	download     *bool
	forceTimings *bool
}

func parseFlags() flags {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Generate OPL3 midi bank\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -c <config> -o <output> [-cache <cache>] [-ini <ini>] [-download]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		ini:          flag.String("ini", "", "Path to ini file to convert to config"),
		cache:        flag.String("cache", "", "Path to cache file"),
		config:       flag.String("c", "", "Path to config file"),
		output:       flag.String("o", "adlmidi.dat", "Path to output file"),
		download:     flag.Bool("download", false, "Download FM banks from github"),
		forceTimings: flag.Bool("force-timings", false, "Recompute WOPL v3 timings and detect silent instruments"),
	}

	flag.Parse()

	return fs
}

func (f flags) flatten() args {
	return args{
		ini:          *f.ini,
		cache:        *f.cache,
		config:       *f.config,
		output:       *f.output,
		download:     *f.download,
		forceTimings: *f.forceTimings,
	}
}

var sighandle = xflags.SigHandle
