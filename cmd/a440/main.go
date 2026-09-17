// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// a440 generates audio signals such as sine waves, sweeps, and impulses.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/erik-adelbert/duh/pkg/a440"
	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/wav"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

const (
	DefaultWavFile  = "a440.wav"
	DefaultFormat   = "s16c2r44100" // p9 pcmconv notation
	DefaultDuration = 30 * time.Second

	// Default to a440
	DefaultSignal    = "sine"
	DefaultFrequency = 440.0 * Hz

	//Impulse parameters
	DefaultImpulseAt = 0 * time.Second

	// Default sweep parameters
	DefaultStart = 440.0 * Hz
	DefaultEnd   = 880.0 * Hz
	DefaultMode  = "logarithmic"
)

type args struct {
	pcm bool

	fmt pcm.Format

	sig string
	out string

	at time.Duration

	dt     time.Duration
	nframe int64

	fq Frequency

	// Sweep parameters
	start Frequency
	end   Frequency
	mode  SweepMode
}

func main() {
	const ExitError = 1

	a := getArgs()

	var r *a440.Generator

	// Verbose output
	verbosef(
		"Generating %v of %.2f (%s) %s into %s...\n",
		a.dt, a.fq, a.fq, a.sig, a.out,
	)

	switch a.sig {
	case "sweep":
		verbosef(
			"Sweep from %.2f to %.2f in %s (%s mode)\n",
			a.start, a.end, a.dt, a.mode,
		)
	case "impulse":
		verbosef("Impulse at %s\n", a.at)
	}

	if a.nframe > 0 {
		verbosef("Frame count: %d\n", a.nframe)
	}

	// Get the appropriate reader based on the signal type
	r = getReader(a)
	defer r.Close() //nolint:errcheck // r.Close() can't fail

	var (
		f   *os.File
		err error
	)

	// Create the output file or use stdout
	f = os.Stdout // Default to stdout

	if !a.pcm && a.out != "-" { // Output file is specified
		f, err = os.Create(a.out)
		die(err, "", ExitError)

		defer f.Close() //nolint:errcheck // can't do anything if it fails
	}

	var w io.Writer = os.Stdout

	if !a.pcm {
		var wv *wav.Encoder

		verbosef("Writing WAV data to %s\n", a.out)

		// Create a WAV writer and write the PCM data
		wv, err = wav.Encode(f, a.fmt)
		die(err, "", ExitError)

		defer func() {
			err = wv.Close()
			die(err, "", ExitError)
		}()

		w = wv
	}

	_, err = io.Copy(w, r)
	die(err, "", ExitError)
}

var sigs = map[string]func(pcm.Format, time.Duration, Frequency) *a440.Generator{
	"sawtooth": a440.NewSawtooth,
	"silence":  a440.NewSilence,
	"sine":     a440.NewSine,
	"square":   a440.NewSquare,
	"triangle": a440.NewTriangle,

	"pink":  a440.NewPink,  // noise
	"white": a440.NewWhite, // noise

	"impulse": nil, // requires at parameter
	"sweep":   nil, // requires start end and mode parameters
}

func getReader(a args) *a440.Generator {
	// special handling for sweep and impulse signals
	switch a.sig {
	case "sweep":
		return a440.NewSweep(
			a.fmt, a.dt, a.start, a.end, a.mode,
		)
	case "impulse":
		return a440.NewImpulse(
			a.fmt, a.dt, a.fq, a.at,
		)
	}

	newReader, _ := sigs[a.sig]

	return newReader(a.fmt, a.dt, a.fq)
}

// CLI arguments and flags

func getArgs() args {
	var err error

	fs := parseFlags()

	a := fs.flatten()

	// Sanitize PCM format
	a.fmt, err = pcm.ParseFormat(*fs.fmt)
	if !a.pcm {
		a.fmt, err = wav.ParseFormat(*fs.fmt)
	}

	if err != nil {
		die(err, *fs.fmt, 2)
	}

	// Sanitize signal type
	_, ok := sigs[a.sig]

	if !ok {
		die(fmt.Errorf("invalid signal type"), a.sig, 2)
	}

	// Sanitize frequency
	nyquist := Frequency(a.fmt.SampleRate()) / 2

	if a.fq <= 0 || a.fq > nyquist {
		err = fmt.Errorf("invalid frequency (0...%v)", nyquist)
		die(err, a.fq.String(), 2)
	}

	// Sanitize durations
	if a.dt < 0 {
		err = errors.New("invalid duration (must be > 0)")
		die(err, a.dt.String(), 2)
	}

	if a.nframe > 0 {
		// convert frame count to duration
		sampleCount := a.nframe * int64(a.fmt.FrameSize())
		a.dt = a.fmt.Duration(sampleCount)
	}

	if a.at < 0 || a.at > a.dt {
		err = fmt.Errorf("invalid duration (0...%s)", a.dt)
		die(err, a.at.String(), 2)
	}

	return a
}

type flags struct {
	pcm *bool

	fmt *string
	sig *string
	out *string

	at *time.Duration

	nframe *int
	dt     *time.Duration

	fq *Frequency

	// Sweep parameters
	start *Frequency
	end   *Frequency
	mode  *string
}

func parseFlags() flags {
	var (
		start = DefaultStart
		end   = DefaultEnd
		fq    = DefaultFrequency
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Generate reference audio signals\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Var(&start, "start", "Sweep start frequency")
	flag.Var(&end, "end", "Sweep end frequency")
	flag.Var(&fq, "hz", "Signal frequency: 1000mHz, 1Hz, 2kHz, ...")

	sigs := slices.Collect(maps.Keys(sigs))

	fs := flags{
		pcm: flag.Bool("pcm", false, "output raw PCM on stdout"),
		fmt: flag.String(
			"of", DefaultFormat,
			"PCM format",
		),
		sig: flag.String(
			"s", DefaultSignal,
			fmt.Sprintf("Signal type: %v", sigs),
		),
		out: flag.String(
			"o", DefaultWavFile,
			"Output WAV file path",
		),
		at: flag.Duration(
			"at", DefaultImpulseAt,
			"Impulse position: 4m, 27s ... (max dt) (default 0s)",
		),
		dt: flag.Duration(
			"dt", DefaultDuration,
			"Signal duration: 4m, 27s ... (max 30m)",
		),
		nframe: flag.Int(
			"n", 0,
			"Frame count: 2, 100, 44100 ... (supersedes dt)",
		),
		fq: &fq,

		start: &start,
		end:   &end,
		mode: flag.String(
			"mode", DefaultMode,
			"Sweep mode: linear, logarithmic",
		),
	}

	flag.Parse()

	return fs
}

func (f flags) flatten() args {

	mode := a440.SweepLinear
	if *f.mode == "logarithmic" {
		mode = a440.SweepLogarithmic
	}

	return args{
		sig:    *f.sig,
		out:    *f.out,
		pcm:    *f.pcm,
		at:     *f.at,
		nframe: int64(*f.nframe),
		dt:     *f.dt,
		fq:     *f.fq,
		start:  *f.start,
		end:    *f.end,
		mode:   mode,
	}
}

// Type and const sweet aliases

type (
	Frequency = a440.Frequency
	SweepMode = a440.SweepMode
)

var (
	die      = xflags.MkDie(os.Stderr)
	verbosef = xflags.MkVerbosef(os.Stderr)
)

const Hz = a440.Hz
