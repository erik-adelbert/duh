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

func NewSawtooth(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	sr := f.SampleRate()
	return NewGenerator(f, duration, freq, sawtooth(freq, float64(sr)))
}

func sawtooth(freq Frequency, sampleRate float64) iter.Seq[float64] {
	var phase float64

	step := freq.Hz() / sampleRate

	return func(yield func(float64) bool) {
		for {
			p := phase - math.Floor(phase)
			x := 2*p - 1

			if !yield(x) {
				return
			}

			phase += step
		}
	}
}
