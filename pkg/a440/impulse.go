// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"iter"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func NewImpulse(f pcm.Format, duration time.Duration, freq Frequency, at time.Duration) *Generator {
	sr := f.SampleRate()
	return NewGenerator(f, duration, freq, impulse(at, sr))
}

func impulse(at time.Duration, sampleRate int) iter.Seq[float64] {
	var done bool

	ncount := 0
	target := int(at.Seconds()) * sampleRate

	return func(yield func(float64) bool) {
		for {
			switch {
			case ncount == target && !done:
				if !yield(1.0) {
					return
				}

				done = true
			default:
				if !yield(0.0) {
					return
				}
			}

			ncount++
		}
	}
}
