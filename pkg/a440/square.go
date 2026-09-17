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

func NewSquare(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	sr := f.SampleRate()
	return NewGenerator(f, duration, freq, square(freq, float64(sr)))
}

func square(freq Frequency, sampleRate float64) iter.Seq[float64] {
	var phase float64

	// step := 2 * math.Pi * freq.Hz() / sampleRate

	return func(yield func(float64) bool) {
		for {
			s := math.Sin(phase)

			x := 1.0
			if s < 0 {
				x = -1.0
			}

			if !yield(x) {
				return
			}

			// phase += step
			phase += 2 * math.Pi * freq.Hz() / sampleRate
		}
	}
}
