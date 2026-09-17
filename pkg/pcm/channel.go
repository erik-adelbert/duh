// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

// channel represents a single audio channel for resampling and filtering.
type channel struct {
	ρ uint32 // fractional position

	t uint32 // time

	δt uint32 // output step
	δl uint32 // step size
	wl uint32 // window length

	u  int32   // unity scale
	h  []int32 // filter coefficients
	δh []int32 //  interpolation delta coefs

	obuf // output buffer
}

// init initializes the channel for resampling from input rate to output rate.
func (c *channel) init(irate, orate, nframe int) (sampleCount int, err error) {
	c.ρ = uint32((uint64(orate) << Np) / uint64(irate))

	if c.ρ == One { // Identity rate, no resampling needed
		sampleCount = nframe
		return
	}

	c.δt = uint32((uint64(irate) << Np) / uint64(orate))

	c.δl = 1 << (Nl + Nη)
	c.wl = uint32(len(h) << Nη)
	c.w = 1 + int(c.wl/c.δl)

	c.u = FUScale

	if c.ρ < One {
		c.u *= int32(c.ρ)
		c.u >>= Np

		c.δl *= c.ρ
		c.δl >>= Np

		c.w *= int(c.δt)
		c.w >>= Np
	}

	c.h, c.δh = FIRCoefs()

	c.i = c.w
	c.n = c.w*2 + nframe
	c.t = uint32(c.i << Np)

	bufferSize := c.w*2 + nframe

	c.n = bufferSize
	c.x = make([]int32, c.n)

	sampleCount = int(
		(uint64(nframe+bufferSize) * uint64(c.ρ)) >> Np,
	)

	return
}

// filter applies the FIR filter to the buffered samples and returns the filtered output.
func (c *channel) filter() int32 {

	h, δh := c.h, c.δh
	wl, δl := c.wl, c.δl

	var v int64

	// Left side
	xi := c.t >> Np
	p := c.t & ((1 << Np) - 1)

	l := p
	if c.ρ < One {
		l = (c.ρ * p) >> Np
	}

	for l < wl {
		j := l >> Nη

		a := int32(l & ((1 << Nη) - 1))
		a *= δh[j]
		a >>= Nη
		a += h[j]

		v += int64(c.x[xi]) * int64(a)

		l += δl
		xi--
	}

	// Right side
	xi = c.t >> Np
	p = (One - p) & ((1 << Np) - 1)

	l = p
	if c.ρ < One {
		l = (c.ρ * p) >> Np
	}

	if p == 0 {
		l += δl
	}

	for l < wl {
		j := l >> Nη

		a := int32(l & ((1 << Nη) - 1))
		a *= δh[j]
		a >>= Nη
		a += h[j]

		v += int64(c.x[xi]) * int64(a)

		l += δl
		xi++
	}

	v >>= 2
	v *= int64(c.u)
	v >>= 27

	return clip(v)
}

func (c *channel) reset() {
	c.i = c.w

	c.t = uint32(c.i << Np)

	clear(c.x[:c.n])
}

// resample resamples the input samples and writes the output samples.
func (c *channel) resample(out, in []int32, nframe int) (n int) {
	if c.ρ == One {
		return copy(out, in[:nframe])
	}

	// If no frames to process, just fill the tail with zeros.
	if nframe == 0 {
		need := 2 * c.w
		tail := c.x[c.i : c.i+need]

		clear(tail)
		c.i += need
	}

	for {
		// Fill buffer
		sz := min(nframe, c.n-c.i)

		if sz > 0 {
			copy(c.x[c.i:c.i+sz], in[:sz])

			c.i += sz
			nframe -= sz
			in = in[sz:]
		}

		// need at least 2*wx samples for FIR filtering
		if c.i < 2*c.w {
			break
		}

		twin := uint32(c.i-c.w) << Np

		for c.t < twin {
			out[n] = c.filter()
			c.t += c.δt
			n++
		}

		cur := c.t >> Np // Current read position (integer part)
		if cur >= uint32(c.n-c.w) {
			// We've consumed too much, need to shift buffer

			// Retain unread samples after a left FIR padding region.
			cur -= uint32(c.w)
			rem := uint32(c.i) - cur
			nxt := uint32(c.w)

			copy(c.x[nxt:nxt+rem], c.x[cur:cur+rem])

			c.t -= cur << Np
			c.t += nxt << Np
			c.i = int(nxt + rem)
		}

		if nframe <= 0 {
			// Nothing left to process
			break
		}
	}

	return
}

// obuf holds buffered audio samples and its current state.
type obuf = struct {
	x []int32 // buffer
	w int     // window size
	i int     // buffer index
	n int     // number of samples in buffer
}
