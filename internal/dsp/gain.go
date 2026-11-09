// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// # Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package dsp

import (
	"math"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

type Gain struct {
	dB   float64
	gain float64
	fmt  pcm.Format

	*pcm.Ring

	win64     []float64
	scratch32 []int32
}

const (
	bufDuration = 200 * ms
	frameCount  = 1024 // Number of *frames* in the processing window
)

func NewGain(dB float64, format pcm.Format) *Gain {
	winsz := frameCount * int(format.FrameSize())

	ring, _ := pcm.NewRing(format, bufDuration)

	return &Gain{
		dB:   dB,
		gain: math.Pow(10.0, dB/20.0),
		fmt:  format,

		win64:     make([]float64, winsz),
		scratch32: make([]int32, winsz),

		Ring: ring,
	}
}

func (g *Gain) Ifmt() pcm.Format { return g.fmt }
func (g *Gain) Ofmt() pcm.Format { return g.fmt }

func (g *Gain) SetGain(dB float64) {
	g.dB = dB
	g.gain = math.Pow(10.0, dB/20.0)
}

func (g *Gain) Gain() float64       { return g.dB }
func (g *Gain) LinearGain() float64 { return g.gain }

func (g *Gain) Ratio(n int) (int, error) {
	return n, nil
}

func (g *Gain) Process(out, in []byte) int {
	n := g.processFrames(out, in)

	return n
}

func (g *Gain) processFrames(out, in []byte) int {
	nsample := g.fmt.DecodeF64(
		g.win64,
		in,
		g.scratch32,
	)

	for i := 0; i < nsample; i++ {
		g.win64[i] *= g.gain
	}

	nsample = g.fmt.EncodeF64(
		out,
		g.win64[:nsample],
		g.scratch32,
	)

	return nsample * int(g.fmt.SampleSize())
}

const ms = time.Millisecond
