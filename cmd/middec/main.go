//go:build !windows

// adlmidi-go: A Go port of ADLMIDI (OPL3 emulation MIDI player)
// Streams raw PCM (S16LE, stereo, 48k) to stdout.
//
// Usage:
//   adlmidi-go [options] <midifilename> [<banknumber> [<numcards> [<numfourops>]]]
//
// Options are parsed with Go's flag package. Since Go's flag package stops
// parsing at the first non-flag argument, all options must come BEFORE the
// MIDI filename.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	mid "github.com/erik-adelbert/duh/pkg/mid"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

// ---------------------------------------------------------------------------
// Audio / timing configuration
// ---------------------------------------------------------------------------

const (
	// Maximum number of OPL3 cards that can be emulated.
	MaxCards = 100
)

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	a := getArgs()
	defer a.input.Close() //nolint:errcheck // can't do anything if it fails

	midi, err := mid.Decode(a.input, a.opts)
	die(err, "Failed to decode MIDI file", 2)
	defer midi.Close()

	if _, err := io.Copy(os.Stdout, midi); err != nil {
		die(err, "Failed to copy decoded audio", 1)
	}
}

var (
	bnknames = mid.BankNames()
	modnames = mid.VolumeModels()
)

type args struct {
	input io.ReadCloser
	opts  mid.DecoderOptions
}

func getArgs() args {
	fs := parseFlags()

	pargs := fs.set.Args()

	path := *fs.path
	if path == "-" && len(pargs) > 0 {
		path = pargs[0]
		pargs = pargs[1:]
	}

	if len(pargs) > 3 {
		err := errors.New("too many positional arguments")
		die(err, "Invalid arguments", 2)
	}

	if len(bnknames) == 0 {
		err := errors.New("no ADL banks are available")
		die(err, "Failed to load banks", 1)
	}

	bank, ncard, n4op := 0, 4, 0
	if len(pargs) > 0 {
		bank = parseInt(pargs[0], "bank number")
		if bank == -1 {
			bank = 0
		}
		if bank < 0 || bank >= len(bnknames) {
			die(fmt.Errorf("bank number may only be 0..%d", len(bnknames)-1), "Invalid bank number", 2)
		}
	}

	if len(pargs) > 1 {
		ncard = parseInt(pargs[1], "card count")
		if ncard < 1 || ncard > MaxCards {
			die(fmt.Errorf("number of cards may only be 1..%d", MaxCards), "Invalid card count", 2)
		}
	}

	nquad, total, err := mid.Melodic4OpCount(bank)
	die(err, "Failed to inspect ADL bank", 1)

	if len(pargs) > 2 {
		n4op = parseInt(pargs[2], "four-op count")
	}
	if n4op < 0 || n4op > 6*ncard {
		die(fmt.Errorf("number of four-op channels may only be 0..%d when %d OPL3 cards are used", 6*ncard, ncard), "Invalid four-op count", 2)
	}
	if n4op == 0 {
		n4op = defaultFourOps(nquad, total, ncard)
	}

	synthFlags := 0
	if *fs.adlDrum {
		synthFlags |= mid.ModeADLPercussion
	}
	if *fs.tremolo {
		synthFlags |= mid.ModeHighTremolo
	}
	if *fs.vibrato {
		synthFlags |= mid.ModeHighVibrato
	}
	if *fs.scaleMods {
		synthFlags |= mid.ModeScaleModulators
	}
	if *fs.softPanning {
		synthFlags |= mid.ModeSoftPanning
	}

	var input io.ReadCloser = io.NopCloser(os.Stdin)
	if path != "-" {
		file, err := os.Open(path)
		die(err, "Failed to open MIDI file", 1)
		input = file
	}

	return args{
		input: input,
		opts: mid.DecoderOptions{
			CardCount: ncard, FourOps: n4op, Flags: synthFlags,
			Bank: bank, VolumeModel: *fs.volumeModel, Loop: *fs.loop,
		},
	}
}

func defaultFourOps(nquad, total uint, ncard int) int {
	if total == 0 {
		return 0
	}

	switch {
	case nquad*8 >= total*7:
		return ncard * 6
	case nquad*8 < total:
		return 0
	case ncard == 1:
		return 1
	default:
		return ncard * 4
	}
}

func parseInt(value, name string) int {
	n, err := strconv.Atoi(value)
	die(err, "Invalid "+name, 2)
	return n
}

type flags struct {
	set         *flag.FlagSet
	path        *string
	adlDrum     *bool
	tremolo     *bool
	vibrato     *bool
	scaleMods   *bool
	softPanning *bool
	volumeModel *int
	loop        *bool
}

func parseFlags() flags {
	set := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	set.Usage = func() {
		fmt.Fprintf(os.Stderr, "Decode MIDI files to raw PCM data\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] [file [bank [cards [four-ops]]]]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		set.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nA four-op count of 0 selects automatically from the bank.\n")
		fmt.Fprintf(os.Stderr, "\nVolume Models:\n")
		for i, name := range modnames {
			fmt.Fprintf(os.Stderr, "%10s%2d = %s\n", "", i, name)
		}
		fmt.Fprintf(os.Stderr, "\nBanks:\n")
		for i, name := range bnknames {
			fmt.Fprintf(os.Stderr, "%10s%2d = %s\n", "", i, name)
		}
	}

	fs := flags{
		set:         set,
		path:        set.String("f", "-", "path to MIDI file"),
		adlDrum:     set.Bool("p", false, "enable adlib percussion mode (use with CMF files)"),
		tremolo:     set.Bool("t", false, "enable tremolo amplification mode"),
		vibrato:     set.Bool("v", false, "enable vibrato amplification mode"),
		scaleMods:   set.Bool("s", false, "enable scaling of modulator volumes"),
		softPanning: set.Bool("sp", false, "enable soft panning (default false)"),
		volumeModel: set.Int("vm", 0, "volume model (default 0)"),
		loop:        set.Bool("lp", false, "loop playback (default false)"),
	}
	if err := set.Parse(os.Args[1:]); err != nil {
		die(err, "Invalid flags", 2)
	}
	return fs
}

var die = xflags.MkDie(os.Stderr)
