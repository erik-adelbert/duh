// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

//go:build js && wasm

package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/erik-adelbert/duh/internal/tui"
	"github.com/erik-adelbert/duh/pkg/mod"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

type webtui struct {
	tap      *pcm.Tap
	spectrum tui.Spectro
	left     tui.VuBar
	right    tui.VuBar
}

func newWebTUI(decoder *mod.Decoder) (io.Reader, *webtui, error) {
	spectrum, err := tui.NewSpectro(48000, 50, 20000, 64, false)
	if err != nil {
		return nil, nil, err
	}
	tap, err := pcm.NewTap(decoder.Format(), 100*time.Millisecond, 2048, 1470)
	if err != nil {
		return nil, nil, err
	}
	ui := &webtui{tap: tap, spectrum: spectrum, left: tui.NewVuBar(), right: tui.NewVuBar()}
	tap.UpdateFunc(func(samples []float64, channels int) {
		for i := 0; i+1 < len(samples); i += channels {
			left, right := samples[i], samples[i+1]
			ui.spectrum.Addf64((left + right) / 2)
			ui.left.Addf64(left)
			ui.right.Addf64(right)
		}
	})
	tap.Start()
	return io.TeeReader(decoder, tap), ui, nil
}

func (ui *webtui) Stop() { ui.tap.Stop() }

func (ui *webtui) Render(seconds float64) string {
	var out strings.Builder
	pad := tui.Mkpad(2)
	ui.left.Tick(time.Now())
	ui.right.Tick(time.Now())
	out.WriteString(tui.ANSICLS)
	fmt.Fprintf(&out, "%s\n  duh modweb\n\n\n", pad)
	_, _ = ui.spectrum.FRender(&out, pad)
	out.WriteString("\n  L ")
	ui.left.Frender(&out, 40)
	out.WriteString("\n  R ")
	ui.right.Frender(&out, 40)
	out.WriteString("\n  -36dB\n\n")
	fmt.Fprintf(&out, "%s%s", pad, tui.TimerString(time.Duration(seconds*float64(time.Second))))
	out.WriteString(tui.Green(" x1\n"))
	return out.String()
}
