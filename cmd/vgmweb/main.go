// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

// vgmweb plays VGM files in the browser (WASM+js) with optional spectrogram and VU bar visualizations.
package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"syscall/js"
	"time"

	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/backend"
	"github.com/erik-adelbert/duh/pkg/pcm"
	vgm "github.com/erik-adelbert/duh/pkg/vgmo3"
)

const (
	DefaultLoopCount = vgm.DefaultLoopCount
	MaxSourceSize    = 50 * MB

	ExitError = 1
)

type Input interface {
	io.Reader
	Size() int64
}

type Printer func(string)

func run(in *vgm.Decoder, out Printer) error {
	in.SetLoopCount(-1)
	return play(out, in)
}

func main() {
	js.Global().Set("run", js.FuncOf(func(this js.Value, args []js.Value) any {
		data := args[0]

		buf := make([]byte, data.Get("length").Int())
		js.CopyBytesToGo(buf, data)

		in, err := vgm.Decode(bytes.NewReader(buf), int64(len(buf)))
		if err != nil {
			return err.Error()
		}

		go func() {
			defer in.Close() //nolint:errcheck

			err := run(in, func(screen string) {
				js.Global().Call("renderScreen", screen)
			})

			if err != nil {
				js.Global().Call("renderError", err.Error())
			}

		}()

		return nil
	}))

	select {}
}

const (
	BandCount = 64
	MinHz     = 50 * Hz
	MaxHz     = 20_000 * Hz
)

func play(out Printer, in *vgm.Decoder) (err error) {
	var (
		sp  tui.Spectro
		vbL tui.VuBar
		vbR tui.VuBar
	)

	var src io.Reader = in

	sp, err = tui.NewSpectro(
		vgm.SampleRateHz, MinHz, MaxHz, BandCount, false,
	)

	if err != nil {
		return err
	}

	vbL = tui.NewVuBar()
	vbR = tui.NewVuBar()

	const (
		WindowSize     = 1024
		BufferDuration = 500 * ms
	)

	// Set up a PCM tap to analyze the audio data for the spectrogram and vubars
	tap, _ := pcm.NewTap(in.Format(), BufferDuration, WindowSize, 0)
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

			sp.Addf64((left + right) / 2)

			vbL.Addf64(left)
			vbR.Addf64(right)
		}
	})
	tap.Start() // Begin tapping the audio data

	src = io.TeeReader(in, tap)

	OPL3Regs := tui.NewOPL3State()

	player, err := backend.NewOtoBackend(src, in.Format(), true)
	if err != nil {
		return err
	}

	defer player.Close()

	// Start playback
	player.Play()
	duration := tui.TimerString(in.Duration())
	started := time.Now()

	// Launch the frame ticker
	const (
		FPS        = 30
		leftMargin = 2
	)

	vsync := time.Second / FPS

	vsyncer := time.NewTicker(vsync)
	defer vsyncer.Stop()

	// Launch the housekeep ticker
	housekeep := time.NewTicker(10 * ms)
	defer housekeep.Stop()

	// Set up the alternate screen buffer
	altScreen(out, true)
	defer altScreen(out, false)

	// Frame state
	var (
		title = "duh vgmweb\n\n"

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
		case <-vsyncer.C: // Draw a frame
			sb.Reset()

			const WidgetWidth = 40

			sb.WriteString(tui.ANSICLS)

			fmt.Fprintf(&sb, "%s\n  %s\n", pads, title)

			_, _ = sp.FRender(&sb, pads)

			sb.WriteString("\n  L ")
			vbL.Frender(&sb, WidgetWidth)
			sb.WriteString("\n  R ")
			vbR.Frender(&sb, WidgetWidth)
			sb.WriteString("\n  ")
			sb.WriteString("-36dB")
			sb.WriteString("\n\n")

			elapsed := time.Since(started)

			fmt.Fprintf(&sb, "%s%s / %s", pads, tui.TimerString(elapsed), duration)

			sb.WriteString(tui.Red(" ∞\n"))

			OPL3Regs.Render(&sb, pads)

			out(sb.String())
		case <-housekeep.C: // Housekeeping tasks
			now := time.Now()

			vbL.Tick(now)
			vbR.Tick(now)

			sb.Reset()

			if st, ok := OPL3.State(); ok {
				OPL3Regs.Update(&sb, st)
			}
		}
	}

	player.Pause()
	player.Close()

	return nil
}

func altScreen(out Printer, on bool) {
	if on {
		out(tui.AltScreen(true))
		return
	}

	out(tui.AltScreen(false))
}

const (
	MB = 1024 * 1024
	ms = time.Millisecond
	Hz = 1
)
