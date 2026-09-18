// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// vgmtui plays VGM files with optional spectrogram and VU bar visualizations.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	// "log"
	// "net/http"
	// _ "net/http/pprof"

	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/backend"
	"github.com/erik-adelbert/duh/pkg/mmcmp"
	"github.com/erik-adelbert/duh/pkg/pcm"
	vgm "github.com/erik-adelbert/duh/pkg/vgmo3"
	"github.com/erik-adelbert/duh/pkg/xflags"
)

const (
	DefaultLoopCount = vgm.DefaultLoopCount
	MaxSourceSize    = 50 * MB

	ExitError = 1
)

func main() {
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// Parse command-line arguments
	a := getArgs()

	// Open the VGM source
	var (
		in  *vgm.Decoder
		err error
	)

	if a.path == "-" {
		// Read from stdin with a limit to prevent excessive memory usage
		verbosef(
			"Reading from stdin with a limit of %d bytes...\n",
			MaxSourceSize+1,
		)

		// mmmcmp does not support streaming decompression,
		// so we need to read the entire input into memory first.
		limited := io.LimitReader(os.Stdin, mmcmp.MaxCompressedSize+1)

		vgms, err := io.ReadAll(limited)
		die(err, "", ExitError)

		vgmsz := int64(len(vgms))

		in, err = vgm.Decode(bytes.NewReader(vgms), vgmsz)
		die(err, "", ExitError)
	} else {
		f, err := os.Open(a.path)
		die(err, "", ExitError)

		defer f.Close() //nolint:errcheck // can't do anything if it fails

		infos, err := f.Stat()
		die(err, "", ExitError)

		// Read from the specified file path
		in, err = vgm.Decode(f, infos.Size())
		die(err, "", ExitError)
	}

	defer in.Close() //nolint:errcheck // can't do anything if it fails

	in.SetLoopCount(a.nloop) // loop playback  -1: ∞

	if a.dump { // -dump flag
		// Dump VGM commands to stdout and exit
		fmt.Print(in.DumpCommands())
		return
	}

	verbosef("Playing %v with %d loops...\n", in, a.nloop)

	oto, err := backend.NewOtoBackend(in.Format(), false)
	die(err, "", ExitError)

	err = play(oto, in, a)
	die(err, "", ExitError)
}

const (
	BandCount = 64
	MinHz     = 50 * Hz
	MaxHz     = 20_000 * Hz
)

func play(oto *backend.OtoBackend, in *vgm.Decoder, a args) (err error) {
	defer closeTTY()

	var (
		sp  tui.Spectro
		vbL tui.VuBar
		vbR tui.VuBar
	)

	var src io.Reader = in

	// Set up the spectrogram and vubars
	if a.spectro || a.vubar {
		if a.spectro {
			sp, err = tui.NewSpectro(
				vgm.SampleRateHz, MinHz, MaxHz, BandCount, false,
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
		tap, _ := pcm.NewTap(in.Format(), BufferDuration, WindowSize, Hop)
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

		src = io.TeeReader(in, tap)
	}

	// Set up the OPL3 state for register display
	var OPL3Regs *tui.OPL3State

	if a.regs {
		OPL3Regs = tui.NewOPL3State()
	}

	player := oto.NewPlayer(src)
	if player == nil {
		return fmt.Errorf("failed to create audio player")
	}

	defer player.Close()

	// Set up signal handling for graceful termination
	ctrlc, stop := sighandle(os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start playback
	player.Play()
	duration := tui.TimerString(in.Duration())
	started := time.Now()

	// Launch the frame ticker
	const (
		FPS        = 30
		leftMargin = 2
	)

	vsync := 250 * ms
	if a.spectro || a.vubar || a.regs {
		vsync = time.Second / FPS
	}

	vsyncer := time.NewTicker(vsync)
	defer vsyncer.Stop()

	// Launch the housekeep ticker
	housekeep := time.NewTicker(10 * ms)
	defer housekeep.Stop()

	// Set up the alternate screen buffer
	altScreen(true)
	defer altScreen(false)

	// Frame state
	var (
		title = filepath.Base(a.path)

		OPL3 = in.Device

		pads = tui.Mkpad(leftMargin)

		sb strings.Builder
	)

	// Preallocate the frame builder
	const FrameSz = 4 * 1024

	sb.Grow(FrameSz)

	// Event loop
	for player.IsPlaying() {
		select {
		case <-ctrlc: // Exit on Ctrl+C
			player.PauseAndStopReading()

			return nil // Exit gracefully on Ctrl+C
		case <-vsyncer.C: // Draw a frame
			sb.Reset()

			const WidgetWidth = 40

			sb.WriteString(tui.ANSICLS)

			fmt.Fprintf(&sb, "%sduh vgmtui - Ctrl+C to exit\n\n  %s\n\n", pads, title)

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

			elapsed := time.Since(started)

			fmt.Fprintf(&sb, "%s%s / %s", pads, tui.TimerString(elapsed), duration)

			if a.nloop >= 0 {
				fmt.Fprintf(&sb, tui.Green("x%d\n\n"), max(a.nloop, 1))
			} else {
				sb.WriteString(tui.Red(" ∞\n\n"))
			}

			if a.regs {
				OPL3Regs.Render(&sb, pads)
			}

			print(sb.String())
		case <-housekeep.C: // Housekeeping tasks
			now := time.Now()

			if a.vubar {
				vbL.Tick(now)
				vbR.Tick(now)
			}

			if a.regs {
				sb.Reset()

				if st, ok := OPL3.State(); ok {
					OPL3Regs.Update(&sb, st)
				}
			}
		}
	}

	player.Pause()
	player.Close()

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
	path *string

	nloop *int

	regs     *bool
	spectrum *bool
	vubar    *bool

	dump *bool
}

func parseFlags() flags {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Play VGM files with visualizations\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	fs := flags{
		path:     flag.String("f", "", "VGM file path"),
		nloop:    flag.Int("n", DefaultLoopCount, "Loop count (-1 for infinite)"),
		regs:     flag.Bool("opl3", false, "Display OPL3 registers"),
		spectrum: flag.Bool("spectro", false, "Display spectrogram"),
		vubar:    flag.Bool("vu", false, "Display vubars"),
		dump:     flag.Bool("dump", false, "Dump VGM commands to stdout and exit"),
	}

	flag.Parse()

	return fs
}

type args struct {
	path string

	nloop int

	regs    bool
	spectro bool
	vubar   bool

	dump bool
}

func getArgs() args {
	fs := parseFlags()

	a := fs.flatten()

	// Determine the input if no -f flag is provided
	if a.path == "" {
		a.path = "-" // Default to stdin

		posargs := flag.Args() // Positional arguments after flags

		if len(posargs) > 0 {
			a.path = posargs[0]
		}
	}

	return a
}

func (fs *flags) flatten() args {
	return args{
		path: *fs.path,

		nloop: *fs.nloop,

		regs:    *fs.regs,
		spectro: *fs.spectrum,
		vubar:   *fs.vubar,

		dump: *fs.dump,
	}
}

// CLI helper functions
var tty, closeTTY = xflags.OpenTTY(os.Stderr)

var (
	die       = xflags.MkDie(tty)
	verbosef  = xflags.MkVerbosef(tty)
	print     = xflags.MkPrint(tty)
	sighandle = xflags.SigHandle
)

const (
	MB = 1024 * 1024
	ms = time.Millisecond
	Hz = 1
)
