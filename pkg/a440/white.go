// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"iter"
	"math/rand/v2"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func NewWhite(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	return NewGenerator(f, duration, freq, white())
}

func white() iter.Seq[float64] {
	return func(yield func(float64) bool) {
		for {
			if !yield(rand.Float64()*2 - 1) {
				return
			}
		}
	}
}
