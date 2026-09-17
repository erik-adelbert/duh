// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/erik-adelbert/duh/internal/help"
	"github.com/erik-adelbert/duh/pkg/analyzer"
)

const DisplayHeight = 16

type fft struct {
	Size, BandCount, MinHz, MaxHz int
}

type Spectro struct {
	*analyzer.Spectrum

	fft

	val analyzer.SpectrumValue

	ruler0 string
	ruler1 string

	emoji bool
}

func NewSpectro(sampleRate int, minHz, maxHz float64, bandCount int, emoji bool) (Spectro, error) {
	const (
		FftSize          = 1024
		DefaultBandCount = 64
		DefaultMinHz     = 40
		DefaultMaxHz     = 20000
		Overlap          = 0.5
	)

	if bandCount == 0 {
		bandCount = DefaultBandCount
	}

	if minHz == 0 {
		minHz = DefaultMinHz
	}

	if maxHz == 0 {
		maxHz = DefaultMaxHz
	}

	bands := analyzer.NewLogBands(
		bandCount, minHz, maxHz, sampleRate, FftSize,
	)

	spectrum, err := analyzer.NewSpectrum(bands, Overlap, FftSize)
	if err != nil {
		return Spectro{}, err
	}

	r0, r1 := LogRuler(bandCount, minHz, maxHz)
	if emoji {
		r0, r1 = LogRuler(bandCount*2, minHz, maxHz)
	}

	return Spectro{
		Spectrum: spectrum,

		fft: fft{
			Size:      FftSize,
			BandCount: bandCount,
			MinHz:     int(minHz),
			MaxHz:     int(maxHz),
		},

		val: analyzer.SpectrumValue{
			Levels: make([]float64, bandCount),
			Peaks:  make([]float64, bandCount),
		},

		ruler0: r0,
		ruler1: r1,

		emoji: emoji,
	}, nil
}

func (sp *Spectro) Render(pads string) string {
	var sb strings.Builder

	_, err := sp.FRender(&sb, pads)

	if err != nil {
		return ""
	}

	return sb.String()
}

func (sp *Spectro) FRender(sb *strings.Builder, pads string) (n int, err error) {

	cur := sp.val

	nxt := sp.Value()

	const α = 0.6 // Smoothing/morphing factor: 1, no smoothing

	if nxt != nil {
		// Blend the new value with the current one.

		for i := range nxt.Levels {
			cur.Levels[i] += α * (nxt.Levels[i] - cur.Levels[i])
			cur.Peaks[i] += α * (nxt.Peaks[i] - cur.Peaks[i])
		}

		nxt.Free() // if you catch it, you free it!
	}

	bands := cur.Levels
	peaks := cur.Peaks

	const (
		h   = DisplayHeight
		dps = 4
	)

	type bandState struct {
		level int
		peak  int
		show  bool
		color Colorizer
	}

	// Precompute the state of each band for rendering
	states := make([]bandState, len(bands))

	for j, level := range bands {
		states[j] = bandState{
			level: int(level*h*dps + .5),
			peak:  int(peaks[j] * h * dps),
			show:  level > .01,
			color: barColorBand(j, len(bands)),
		}
	}

	emptyRune := "🫥"
	peakRune := "🤮"

	blockRunes := []string{
		"🤓",
		"😇",
		"😎",
		"🫣",
		"🙄",
	}

	if !sp.emoji {
		emptyRune = " "
		peakRune = Bold("⣿")

		blockRunes = []string{
			brailleBlock(0),
			brailleBlock(1),
			brailleBlock(2),
			brailleBlock(3),
			brailleBlock(dps),
		}

	}

	// Build formatted frame for the spectrogram display
	for i := h; i > 0; i-- {
		sb.WriteString(pads)

		for _, s := range states {
			bottom := i * dps

			filled := s.level - bottom
			filled = help.Clamp(filled, 0, dps)

			switch {
			case s.peak >= bottom && s.peak < bottom+dps && s.show:
				sb.WriteString(peakRune) // 🤮
			case filled > 0:
				sb.WriteString(s.color(blockRunes[filled])) // 🤓🙄
			default:
				sb.WriteString(emptyRune) // 🫥
			}
		}

		sb.WriteByte('\n')
	}

	sb.WriteString(pads)
	sb.WriteString(sp.ruler0)
	sb.WriteByte('\n')

	sb.WriteString(pads)
	sb.WriteString(sp.ruler1)
	sb.WriteByte('\n')

	return
}

func brailleBlock(filled int) string {
	const (
		brailleBase = 0x2800

		// Braille dot rows, from bottom to top, use bits 7/8, 3/6, 2/5, and 1/4.
		masks = "\x00\xc0\xe4\xf6\xff"
	)

	return string(rune(brailleBase | int(masks[filled])))
}

type logRuler struct {
	minHzStr string
	maxHzStr string

	minLen int
	maxLen int

	minHz int
	maxHz int
}

func newLogRuler(minFreq, maxFreq int) *logRuler {
	minHzStr := strconv.Itoa(minFreq)
	maxKHzStr := strconv.Itoa(maxFreq / 1000)

	return &logRuler{
		minHzStr: minHzStr + "Hz",
		maxHzStr: maxKHzStr + "kHz",

		minLen: len(minHzStr) + 2,  // +2 for "Hz"
		maxLen: len(maxKHzStr) + 3, // +3 for "KHz"

		minHz: minFreq,
		maxHz: maxFreq,
	}
}

func (f *logRuler) Render(n int) (string, string) {

	// Logarithmic ruler
	ruler := make([]byte, n)
	for i := range ruler {
		ruler[i] = ' '
	}

	logMin := math.Log(float64(f.minHz))
	logMax := math.Log(float64(f.maxHz))

	pos := func(fq float64) int {
		x := (math.Log(fq) - logMin) / (logMax - logMin)
		return int(math.Round(x * float64(n-1)))
	}

	fq0 := math.Pow(10, math.Floor(math.Log10(float64(f.minHz))))

	for fq10 := fq0; fq10 <= float64(f.maxHz); fq10 *= 10 { // decade
		for i := 1; i <= 9; i++ {
			fq := fq10 * float64(i) // decade fraction

			if fq < float64(f.minHz) || fq > float64(f.maxHz) {
				continue
			}

			p := pos(fq)
			if p >= 0 && p < n {
				ruler[p] = '|'
			}
		}
	}

	// Labels
	var pads = " "

	if totLen := f.minLen + f.maxLen; totLen < n {
		nspace := n - totLen
		pads = Mkpad(nspace)
	}

	return string(ruler), fmt.Sprintf("%s%s%s", f.minHzStr, pads, f.maxHzStr)
}

func LogRuler(width int, minHz, maxHz float64) (string, string) {
	r := newLogRuler(int(minHz), int(maxHz))

	return r.Render(width)
}
