// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"iter"
	"math/bits"
	"math/rand/v2"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func NewPink(f pcm.Format, duration time.Duration, freq Frequency) *Generator {
	return NewGenerator(f, duration, freq, pink())
}

func pink() iter.Seq[float64] {
	p := newPink()

	return func(yield func(float64) bool) {
		for {
			if !yield(p.next()) {
				return
			}
		}
	}
}

type pinkNoise struct {
	rows    [16]float64
	rnd     *rand.Rand
	counter uint32
}

func newPink() *pinkNoise {
	return &pinkNoise{
		rnd: rand.New(rand.NewPCG(1, 2)),
	}
}

func (p *pinkNoise) next() float64 {
	p.counter++

	// Number of trailing zeroes determines which row changes.
	i := bits.TrailingZeros32(p.counter)

	nrows := len(p.rows)
	if i < nrows {
		p.rows[i] = p.rnd.Float64()*2 - 1
	}

	var sum float64
	for _, v := range p.rows {
		sum += v
	}

	return sum / float64(nrows)
}
