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
	"github.com/erik-adelbert/duh/pkg/opl3"
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

func run(screen Printer, audio *JSOut, in *vgm.Decoder) error {
	in.SetLoopCount(1)

	src, ui := mktui(in)
	defer ui.Stop()

	quit := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		err := play(src, audio)
		close(quit)

		done <- err
	}()

	if err := display(screen, ui, quit); err != nil {
		return err
	}

	return <-done
}

func main() {
	audio := NewJSOut()

	js.Global().Set("run", js.FuncOf(func(this js.Value, args []js.Value) any {
		data := args[0]

		buf := make([]byte, data.Get("length").Int())
		js.CopyBytesToGo(buf, data)

		go func() {
			in, err := vgm.Decode(bytes.NewReader(buf), int64(len(buf)))
			if err != nil {
				js.Global().Call("renderError", err.Error())
				return
			}
			defer in.Close() //nolint:errcheck

			err = run(
				func(screen string) {
					js.Global().Call("renderScreen", screen)
				},
				audio,
				in,
			)

			if err != nil {
				js.Global().Call("renderError", err.Error())
			}

			js.Global().Call("playbackEnded")
		}()

		return nil
	}))

	select {}
}

type webtui struct {
	duration string

	tap *pcm.Tap

	sp   tui.Spectro
	vbL  tui.VuBar
	vbR  tui.VuBar
	regs *tui.OPL3State

	opl3 *opl3.Device
}

// mktui creates a webtui instance for the given VGM decoder and returns an io.Reader
// that taps the audio data for analysis.
func mktui(in *vgm.Decoder) (io.Reader, *webtui) {
	const (
		BandCount = 64
		MinHz     = 50 * Hz
		MaxHz     = 20_000 * Hz
	)

	var (
		err error

		sp  tui.Spectro
		vbL tui.VuBar
		vbR tui.VuBar
	)

	var src io.Reader = in

	sp, err = tui.NewSpectro(
		vgm.SampleRateHz, MinHz, MaxHz, BandCount, false,
	)

	if err != nil {
		return in, nil
	}

	vbL = tui.NewVuBar()
	vbR = tui.NewVuBar()

	OPL3Regs := tui.NewOPL3State()

	const (
		WindowSize     = 2048
		HopSize        = 1470
		BufferDuration = 500 * ms
	)

	// Set up a PCM tap to analyze the audio data for the spectrogram and vubars
	tap, _ := pcm.NewTap(in.Format(), BufferDuration, WindowSize, HopSize)
	// defer tap.Stop() // Removed defer stop to allow manual control via webtui.Stop()

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

	return src, &webtui{
		tap: tap,

		sp:   sp,
		vbL:  vbL,
		vbR:  vbR,
		regs: OPL3Regs,

		opl3: in.Device,

		duration: tui.TimerString(in.Duration()),
	}
}

func (w *webtui) Stop() {
	w.tap.Stop()
}

func play(in io.Reader, audio *JSOut) (err error) {
	buf := make([]byte, 16*1024)

	const (
		sampleRate    = 44100
		bytesPerFrame = 4 // stereo s16le
	)

	var naudio int64

	start := time.Now()

	bps := int64(sampleRate * bytesPerFrame)

	for {
		n, err := in.Read(buf)

		if n > 0 {
			if _, err := audio.Write(buf[:n]); err != nil {
				return err
			}

			naudio += int64(n)

			/*
			 * Keep the decoder approximately synchronized
			 * with real playback time.
			 */
			target := time.Duration(naudio * int64(time.Second) / bps)

			if wait := target - time.Since(start); wait > 2*time.Millisecond {
				time.Sleep(wait)
			}
		}

		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
	}
}

func display(screen Printer, w *webtui, quit chan struct{}) (err error) {
	sp := w.sp
	vbL := w.vbL
	vbR := w.vbR

	opl3 := w.opl3
	OPL3Regs := w.regs

	duration := w.duration

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
	altScreen(screen, true)
	defer altScreen(screen, false)

	// Frame state
	var (
		title = "duh vgmweb\n\n"

		pads = tui.Mkpad(leftMargin)

		sb strings.Builder
	)

	// Preallocate the frame builder
	const FrameSz = 4 * 1024

	sb.Grow(FrameSz)

	// Event loop
	for {
		select {
		case <-quit: // Exit the display loop
			return nil
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

			sb.WriteString(tui.Green("x1\n"))

			OPL3Regs.Render(&sb, pads)

			screen(sb.String())
		case <-housekeep.C: // Housekeeping tasks
			now := time.Now()

			vbL.Tick(now)
			vbR.Tick(now)

			sb.Reset()

			if st, ok := opl3.State(); ok {
				OPL3Regs.Update(&sb, st)
			}
		}
	}
}

func altScreen(out Printer, on bool) {
	if on {
		out(tui.AltScreen(true))
		return
	}

	out(tui.AltScreen(false))
}

type JSOut struct {
	write js.Value
}

func NewJSOut() *JSOut {
	return &JSOut{
		write: js.Global().Get("writeAudio"),
	}
}

func (o *JSOut) Write(p []byte) (int, error) {
	buf := js.Global().Get("Uint8Array").New(len(p))
	js.CopyBytesToJS(buf, p)

	o.write.Invoke(buf)

	return len(p), nil
}

const (
	MB = 1024 * 1024
	ms = time.Millisecond
	Hz = 1
)
