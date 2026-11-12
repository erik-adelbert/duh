// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build js && wasm

// midweb plays MIDI files in the browser with a spectrogram and VU meters.
package main

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"strings"
	"syscall/js"
	"time"

	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/mid"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

const (
	MaxSourceSize = 10 * MB
	MaxCards      = 4
)

type Printer func(string)

func main() {
	audio := NewJSOut()

	names := mid.BankNames()
	jsNames := js.Global().Get("Array").New(len(names))
	for i, name := range names {
		jsNames.SetIndex(i, name)
	}
	js.Global().Call("populateBanks", jsNames)

	js.Global().Set("run", js.FuncOf(func(_ js.Value, args []js.Value) any {
		if len(args) < 2 {
			js.Global().Call("renderError", "Choose a MIDI file and bank before playing.")
			js.Global().Call("playbackEnded")
			return nil
		}

		data, bank := args[0], args[1].Int()
		buf := make([]byte, data.Get("length").Int())
		js.CopyBytesToGo(buf, data)

		go func() {
			if len(buf) > MaxSourceSize {
				js.Global().Call("renderError", "MIDI files must be 10 MiB or smaller.")
				js.Global().Call("playbackEnded")
				return
			}

			nquad, total, err := mid.Melodic4OpCount(bank)
			if err != nil {
				js.Global().Call("renderError", err.Error())
				js.Global().Call("playbackEnded")
				return
			}

			decoder, err := mid.Decode(bytes.NewReader(buf), mid.DecoderOptions{
				CardCount: MaxCards,
				FourOps:   defaultFourOps(nquad, total, MaxCards),
				Bank:      bank,
			})
			if err != nil {
				js.Global().Call("renderError", err.Error())
				js.Global().Call("playbackEnded")
				return
			}
			defer decoder.Close() //nolint:errcheck // can't do anything if it fails
			js.Global().Call("playbackStarted")

			err = run(
				func(screen string) {
					js.Global().Call("renderScreen", screen)
				},
				audio,
				decoder,
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

func run(screen Printer, audio *JSOut, in *mid.Decoder) error {
	src, ui := mktui(in)
	defer ui.Stop()

	quit := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		err := play(src, audio)
		close(quit)

		done <- err
	}()

	if err := display(screen, ui, quit, audio); err != nil {
		return err
	}
	return <-done
}

type webtui struct {
	duration string

	tap *pcm.Tap

	sp  tui.Spectro
	vbL tui.VuBar
	vbR tui.VuBar
}

func mktui(in *mid.Decoder) (io.Reader, *webtui) {
	const (
		BandCount = 64
		MinHz     = 50
		MaxHz     = 20_000
		Window    = 2048
	)

	var (
		err error

		sp       tui.Spectro
		vbL, vbR tui.VuBar
	)

	var src io.Reader = in

	sp, err = tui.NewSpectro(mid.PCMRate, MinHz, MaxHz, BandCount, false)

	if err != nil {
		return in, nil
	}

	vbL, vbR = tui.NewVuBar(), tui.NewVuBar()

	const (
		HopSize = 1470

		BufferDuration = 100 * ms
	)

	tap, _ := pcm.NewTap(in.Format(), BufferDuration, Window, HopSize)

	tap.UpdateFunc(func(samples []float64, nchan int) {
		for i := 0; i < len(samples); i += nchan {
			left := samples[i]
			right := left

			if nchan == 2 {
				right = samples[i+1]
			}

			sp.Addf64((left + right) / 2)

			vbL.Addf64(left)
			vbR.Addf64(right)
		}
	})
	tap.Start()

	src = io.TeeReader(in, tap)

	return src, &webtui{
		duration: tui.TimerString(in.Duration()),

		tap: tap,

		sp:  sp,
		vbL: vbL,
		vbR: vbR,
	}
}

func (w *webtui) Stop() { w.tap.Stop() }

func play(in io.Reader, audio *JSOut) error {
	buf := make([]byte, 16*1024)

	const (
		sampleRate    = 48000
		bytesPerFrame = 4 // stereo s16le
	)

	var naudio int64

	start := audio.Elapsed()

	bps := int64(sampleRate * bytesPerFrame)

	for {
		n, err := in.Read(buf)

		if n > 0 {
			if _, err := audio.Write(buf[:n]); err != nil {
				return err
			}

			naudio += int64(n)

			target := time.Duration(naudio * int64(time.Second) / bps)

			if wait := target - (audio.Elapsed() - start); wait > 2*time.Millisecond {
				time.Sleep(wait)
			} else {
				runtime.Gosched() // behind schedule: wasm has one thread, so let the display run
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

func display(screen Printer, w *webtui, quit <-chan struct{}, audio *JSOut) error {
	const (
		FPS        = 30
		LeftMargin = 2
	)

	vsync := time.NewTicker(time.Second / FPS)
	defer vsync.Stop()

	housekeep := time.NewTicker(10 * time.Millisecond)
	defer housekeep.Stop()

	altScreen(screen, true)
	defer altScreen(screen, false)

	var (
		title = "duh midweb\n\n"

		pads = tui.Mkpad(LeftMargin)

		sb strings.Builder
	)

	for {
		select {
		case <-quit:
			return nil
		case <-vsync.C:
			sb.Reset()

			const WidgetWidth = 40

			sb.WriteString(tui.ANSICLS)

			fmt.Fprintf(&sb, "%s\n  %s\n", pads, title)

			_, _ = w.sp.FRender(&sb, pads)

			sb.WriteString("\n  L ")
			w.vbL.Frender(&sb, WidgetWidth)
			sb.WriteString("\n  R ")
			w.vbR.Frender(&sb, WidgetWidth)
			sb.WriteString("\n  -36dB\n\n")

			fmt.Fprintf(&sb, "%s%s / %s", pads, tui.TimerString(audio.Elapsed()), w.duration)
			sb.WriteString(tui.Green("x1\n"))

			screen(sb.String())
		case now := <-housekeep.C:
			w.vbL.Tick(now)
			w.vbR.Tick(now)
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

type JSOut struct{ write js.Value }

func NewJSOut() *JSOut { return &JSOut{write: js.Global().Get("writeAudio")} }

func (o *JSOut) Elapsed() time.Duration {
	return time.Duration(js.Global().Call("playbackTime").Float() * float64(time.Second))
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
)
