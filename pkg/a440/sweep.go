// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"iter"
	"math"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func NewSweep(f pcm.Format, duration time.Duration, start, end Frequency, mode SweepMode) *Generator {
	sr := int64(f.SampleRate())

	nframe := duration.Nanoseconds() * sr / int64(time.Second)

	return NewGenerator(f, duration, start, sweep(start, end, nframe, float64(sr), mode))
}

func sweep(start, end Frequency, nframe int64, sampleRate float64, mode SweepMode) iter.Seq[float64] {
	var (
		pos   int64
		phase float64
	)

	return func(yield func(float64) bool) {
		for pos < nframe {
			t := float64(pos) / float64(nframe-1)

			// Logarithmic frequency interpolation.
			freq := start.Hz() * math.Pow(end.Hz()/start.Hz(), t)

			if mode == SweepLinear {
				freq = start.Hz() + (end.Hz()-start.Hz())*t
			}

			x := math.Sin(phase)

			phase += 2 * math.Pi * freq / sampleRate
			pos++

			if !yield(x) {
				return
			}
		}
	}
}

type SweepMode uint8

const (
	SweepLinear SweepMode = iota
	SweepLogarithmic
)

func ParseSweepMode(s string) (mode SweepMode) {
	mode = SweepLogarithmic

	if s == "linear" {
		mode = SweepLinear
	}

	return
}

func (m SweepMode) String() string {
	if m == SweepLinear {
		return "linear"
	}

	return "logarithmic"
}
