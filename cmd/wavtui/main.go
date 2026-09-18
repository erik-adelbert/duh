// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// wavtui plays WAV files with optional spectrogram and VU bar visualizations.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/erik-adelbert/duh/internal/player"
	"github.com/erik-adelbert/duh/pkg/backend"
	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/wav"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

// wavtui plays WAV files with optional spectrogram and VU bars.
func main() {
	close := openTTY()
	defer close()

	a := getArgs()

	src, err := os.Open(a.wavPath)
	die(err, "Failed to open WAV file", 1)

	defer src.Close() //nolint:errcheck // can't do anything if it fails

	var in io.Reader

	wave, err := wav.Decode(src)
	die(err, "Failed to create WAV reader", 1)

	in = wave // as io.ReadSeeker

	// Downconversion for unsupported bit depths
	ifmt := wave.Format()
	nch := ifmt.ChannelCount()

	if nch == 0 {
		err := errors.New("no audio channel")
		die(err, "", 1)
	}

	// Create a new PCM-aware tap with a frame buffer of 1024 frames
	// presenting them at 30FPS (default rate)
	const (
		WindowSize = 2048
		Hop        = 1470
	)

	tap, err := pcm.NewTap(ifmt, 500*time.Millisecond, WindowSize, Hop)
	die(err, "Failed to create PCM tap", 1)

	in = pcm.OpenTap(in, tap)

	// Create the backend for audio playback using the taped reader
	ob, err := backend.NewOtoBackend(in, ifmt, false)
	die(err, "Failed to create oto backend", 1)

	defer ob.Close()

	if ob.Format() != ifmt {
		verbosef("Input converted from", ifmt, "to", ob.Format())
	}

	// Create the playback controller
	ctrl := player.NewController(ob)

	name := filepath.Base(a.wavPath)

	// Initialize the player tui
	tui := player.NewTUI(ctrl, name, tap, a.vubar, a.spectrum, a.emoji)

	// Pack everything up
	cli := tea.NewProgram(tui, tea.WithAltScreen())

	_, err = cli.Run()
	die(err, "Failed to run player", 1)
}

type flags struct {
	wavPath  *string
	vubar    *bool
	spectrum *bool
	emoji    *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Play WAV files with a TUI player\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	f := flags{
		wavPath:  flag.String("f", "", "path to the WAV file"),
		vubar:    flag.Bool("vu", false, "enable VU meter"),
		spectrum: flag.Bool("spectro", false, "enable spectro"),
		emoji:    flag.Bool("emoji", false, "if spectro then emoji mode"),
	}

	flag.Parse()

	if *f.wavPath == "" {
		flag.Usage()
		os.Exit(2)
	}

	return f
}

func (f flags) flatten() args {
	return args{
		wavPath:  *f.wavPath,
		vubar:    *f.vubar,
		spectrum: *f.spectrum,
		emoji:    *f.emoji,
	}
}

type args struct {
	wavPath  string
	vubar    bool
	spectrum bool
	emoji    bool
}

func getArgs() args {
	fs := parseFlags()

	return fs.flatten()
}

var tty = os.Stderr

func openTTY() (close func()) {
	x, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return func() {}
	}

	tty = x
	return func() {
		_ = tty.Close()
	}
}

var (
	die      = xflags.MkDie(tty)
	verbosef = xflags.MkVerbosef(tty)
)
