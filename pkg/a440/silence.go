// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"iter"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func NewSilence(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	return NewGenerator(f, duration, freq, silence())
}

func silence() iter.Seq[float64] {
	return func(yield func(float64) bool) {
		for {
			if !yield(0) {
				return
			}
		}
	}
}
