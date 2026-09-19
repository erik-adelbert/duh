// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// pcmtui plays PCM audio data with optional spectrogram and VU bar visualizations.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	// "log"
	// "net/http"
	// _ "net/http/pprof"

	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/backend"
	"github.com/erik-adelbert/duh/pkg/pcm"
	vgm "github.com/erik-adelbert/duh/pkg/vgmo3"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

const (
	DefaultLoopCount = vgm.DefaultLoopCount
	MaxSourceSize    = 50 * MB

	ExitError = 1
)

type args struct {
	ifmt pcm.Format

	vubar   bool
	spectro bool

	src  io.ReadCloser
	path string
	dt   string

	through bool

	emoji bool
}

func main() {
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// Parse command-line arguments

	close := openTTY()
	defer close()

	a := getArgs()
	defer a.src.Close() //nolint:errcheck // can't do anything if it fails

	verbosef("Playing input format %s...from %v\n", a.ifmt, a.path)

	var in io.Reader = a.src

	if a.through {
		in = io.TeeReader(a.src, os.Stdout)
		verbosef("Passthrough mode enabled\n")
	}

	oto, err := backend.NewOtoBackend(a.ifmt, false)
	die(err, "Failed to initialize audio backend", ExitError)

	err = play(oto, in, a)
	die(err, "Failed to play PCM data", ExitError)
}

// Spectro constants
const (
	BandCount = 64
	MinHz     = 50 * Hz
	MaxHz     = 20_000 * Hz
)

func play(oto *backend.OtoBackend, in io.Reader, a args) (err error) {
	var (
		sp  tui.Spectro
		vbL tui.VuBar
		vbR tui.VuBar
	)

	// Set up the spectro and vubars
	if a.spectro || a.vubar {
		if a.spectro {
			sp, err = tui.NewSpectro(
				vgm.SampleRateHz, MinHz, MaxHz, BandCount, a.emoji,
			)

			if err != nil {
				return err
			}
		}

		if a.vubar {
			vbL = tui.NewVuBar()
			vbR = tui.NewVuBar()
		}

		const (
			WindowSize     = 2048
			Hop            = 1470
			BufferDuration = 500 * ms
		)

		// Set up a PCM tap to analyze the audio data for the spectrogram and vubars
		tap, _ := pcm.NewTap(a.ifmt, BufferDuration, WindowSize, Hop)
		defer tap.Stop()

		// Set up the tap's update function to feed the analyzers
		tap.UpdateFunc(func(samples []float64, nchan int) {
			var left, right float64

			for i := 0; i < len(samples); i += nchan {
				switch nchan {
				case 1:
					left = samples[i]
					right = left
				case 2:
					left = samples[i]
					right = samples[i+1]
				}

				if a.spectro {
					sp.Addf64((left + right) / 2)
				}

				if a.vubar {
					vbL.Addf64(left)
					vbR.Addf64(right)
				}
			}
		})
		tap.Start() // Begin tapping the audio data

		in = io.TeeReader(in, tap)
	}

	// Set up the audio playback
	player := oto.NewPlayer(in)
	if player == nil {
		return fmt.Errorf("failed to create audio player")
	}

	// Set up signal handling for graceful termination
	ctrlc, stop := sighandle(os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start playback
	player.Play()
	started := time.Now()

	// Launch the frame ticker
	const (
		FPS        = 30
		leftMargin = 2
	)

	vsync := 250 * ms
	if a.spectro || a.vubar {
		vsync = time.Second / FPS
	}

	vsyncer := time.NewTicker(vsync)
	defer vsyncer.Stop()

	// Launch the housekeep ticker
	housekeep := time.NewTicker(20 * ms)
	defer housekeep.Stop()

	// Set up the alternate screen buffer
	altScreen(true)
	defer altScreen(false)

	// Frame state
	var (
		title = fmt.Sprintf("Playing %s from %s", a.ifmt, a.path)

		pads = tui.Mkpad(leftMargin)

		sb strings.Builder
	)

	// Preallocate the frame builder
	const FrameSz = 4 * kB

	sb.Grow(FrameSz)

	// Event loop
	for player.IsPlaying() {
		select {
		case <-ctrlc: // Exit on Ctrl+C
			player.Pause()

			return nil // Exit gracefully on Ctrl+C
		case <-vsyncer.C: // Draw a frame
			sb.Reset()

			const WidgetWidth = 40

			sb.WriteString(tui.ANSICLS)

			fmt.Fprintf(&sb, "%sduh pcmplay - Ctrl+C to exit\n\n  %s\n\n", pads, title)

			elapsed := time.Since(started)

			fmt.Fprintf(&sb, "%s%s / %s\n", pads, tui.TimerString(elapsed), a.dt)

			if a.spectro {
				_, _ = sp.FRender(&sb, pads)
			}

			if a.vubar {
				sb.WriteString("\n  L ")
				vbL.Frender(&sb, WidgetWidth)
				sb.WriteString("\n  R ")
				vbR.Frender(&sb, WidgetWidth)
				sb.WriteString("\n  ")
				sb.WriteString("-36dB")
				sb.WriteString("\n\n")
			}

			print(sb.String())
		case <-housekeep.C: // Housekeeping tasks
			now := time.Now()

			if a.vubar {
				vbL.Tick(now)
				vbR.Tick(now)
			}
		}
	}

	player.PauseAndStopReading()

	return nil
}

func altScreen(on bool) {
	if on {
		print(tui.AltScreen(true))
		return
	}

	print(tui.AltScreen(false))
}

// Command-line argument parsing

type flags struct {
	vubar    *bool
	spectrum *bool
	path     *string
	through  *bool
	emoji    *bool

	ifmt *string
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Play PCM audio data with visualizations\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		ifmt:     flag.String("if", "", "Input format (s16c2r44100)"),
		path:     flag.String("i", "-", "path to MP3 file (default: stdin)"),
		through:  flag.Bool("p", false, "Enable PCM passthrough mode"),
		vubar:    flag.Bool("vu", false, "Display VU bars"),
		spectrum: flag.Bool("spectro", false, "Display spectrogram"),
		emoji:    flag.Bool("emoji", false, "if spectro then emoji mode"),
	}

	flag.Parse()

	return fs
}

func getArgs() args {
	var (
		a   args
		err error
	)

	fs := parseFlags()

	if *fs.ifmt == "" {
		err = errors.New("input format is required")
		die(err, "Input format is required", 2)
	}

	a.ifmt, err = pcm.ParseFormat(*fs.ifmt)
	die(err, "Failed to parse input format", 2)

	a.spectro = *fs.spectrum
	a.emoji = *fs.emoji
	a.vubar = *fs.vubar
	a.through = *fs.through

	if *fs.path == "-" {
		a.path = "stdin"
		a.src = io.NopCloser(os.Stdin)
		a.dt = tui.Red("∞")
	} else {
		a.path = filepath.Base(*fs.path)

		a.src, err = os.Open(*fs.path)
		die(err, "Failed to open input file", 1)

		infos, err := os.Stat(*fs.path)
		die(err, "Failed to stat input file", 1)

		dt := a.ifmt.Duration(infos.Size())

		a.dt = tui.TimerString(dt)
	}

	return a
}

// CLI helper functions

func sighandle(s ...os.Signal) (<-chan os.Signal, func()) {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, s...)

	return sigs, func() {
		signal.Stop(sigs)
	}
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
	print    = xflags.MkPrint(tty)
)

const (
	kB = 1024
	MB = 1024 * 1024
	ms = time.Millisecond
	Hz = 1
)
