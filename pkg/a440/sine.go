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

func NewSine(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	sr := f.SampleRate()
	return NewGenerator(f, duration, freq, sine(freq, float64(sr)))
}

func sine(freq Frequency, sampleRate float64) iter.Seq[float64] {
	var phase float64

	step := 2 * math.Pi * freq.Hz() / sampleRate

	return func(yield func(float64) bool) {
		for {
			if !yield(math.Sin(phase)) {
				return
			}

			phase += step
		}
	}
}
