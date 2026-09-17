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

import (
	"cmp"
	"encoding/binary"
	"math"
	"slices"

	"golang.org/x/exp/constraints"
)

// Converter is a PCM converter that converts audio data from one format to another.
type Converter struct {
	ifmt, ofmt Format

	iconv inner
	oconv outer

	in, out []int32

	buf []byte

	seed uint32 // random dithering seed

	flags int32

	chans []channel
}

// NewConverter creates a new PCM converter
func NewConverter(ofmt, ifmt Format) (c *Converter, err error) {
	defer func() {
		err = mkError(ErrNew, err)
	}()

	c = new(Converter)

	if ifmt == ofmt {
		c.flags |= Fcopy
	}

	if ifmt.channelCount < 1 || ofmt.channelCount < 1 {
		err = mkError(errBadFormat, "wrong channel count")
		return
	}

	c.ifmt, c.ofmt = ifmt, ofmt

	c.chans = make([]channel, ifmt.channelCount)

	c.buf = make([]byte, 0, ifmt.frameSize)

	i := slices.IndexFunc(
		iconvs,
		func(x iconf) bool { return ifmt.tag == x.fmt },
	)

	if i == -1 {
		err = mkError(errBadFormat, "bad input format")
		return
	}

	c.iconv = iconvs[i].fun

	i = slices.IndexFunc(
		oconvs,
		func(x oconf) bool { return ofmt.tag == x.fmt },
	)

	if i == -1 {
		err = mkError(errBadFormat, "bad output format")
		return
	}

	c.oconv = oconvs[i].fun

	c.in = make([]int32, MaxFrame)

	var outsz int

	for i := range c.chans {
		outsz, err = c.chans[i].init(ifmt.sampleRate, ofmt.sampleRate, MaxFrame)

		if err != nil {
			return
		}
	}

	// out is used both as resampler and mix buffer
	outsz = max(outsz, MaxFrame)
	c.out = make([]int32, outsz)

	return
}

func (c *Converter) Ifmt() Format {
	return c.ifmt
}

func (c *Converter) Ofmt() Format {
	return c.ofmt
}

// Reset resets the converter to its initial state.
func (c *Converter) Reset() {
	c.buf = c.buf[:0]

	for i := range c.chans {
		c.chans[i].reset()
	}
}

// Process converts audio data from the input format to the output format.
func (c *Converter) Process(out []byte, in []byte) (n int) {

	if has(c.flags, Fcopy) {
		n = copy(out, in)
		return
	}

	ifmt := c.ifmt

	todo := len(in)

	// handle any buffered input data first
	if len(c.buf) > 0 {
		nfree := ifmt.frameSize - len(c.buf) // remaining space in buffer
		sz := min(nfree, todo)               // amount of data to copy into the buffer

		// copy data into the buffer
		c.buf = append(c.buf, in[:sz]...)

		in = in[sz:] // advance the input slice

		todo -= sz

		if len(c.buf) < ifmt.frameSize {
			// not enough data to form a complete frame, wait for more input
			return
		}

		nconv := c.conv(out, c.buf[:ifmt.frameSize])
		n += int(nconv)

		out = out[nconv:]
		c.buf = c.buf[:0] // remove the processed frame from the buffer
	}

	// calculate the number of complete frames in the input
	frac := todo % ifmt.frameSize
	todo -= frac

	n += c.conv(out, in[:todo]) // convert the complete frames

	// handle any remaining fractional input data
	if frac > 0 {
		c.buf = append(c.buf, in[todo:todo+frac]...)
	}

	return
}

// Flush flushes any buffered input data to the output format.
func (c *Converter) Flush(out []byte) (n int) {

	if len(out) != 0 {
		if has(c.flags, Fcopy) {
			return copy(out, c.buf)
		}

		if len(c.buf) > 0 {
			frame := make([]byte, c.ifmt.frameSize)

			copy(frame, c.buf)
			n += c.conv(out, frame)
		} else {
			n += c.conv(out, nil)
		}
	}

	c.buf = c.buf[:0]
	return
}

// Ratio returns the number of output bytes that will be produced for the given
// number of input bytes.
func (c *Converter) Ratio(inSize int) (outSize int, err error) {
	defer func() {
		err = mkError(ErrRatio, err)
	}()

	ifsz := uint64(c.ifmt.frameSize)
	ofsz := uint64(c.ofmt.frameSize)

	if uint64(inSize) < ifsz {
		err = mkError(errInvalidValue, "input is too small")
		return
	}

	irate, orate := uint64(c.ifmt.sampleRate), uint64(c.ofmt.sampleRate)

	if irate == 0 {
		err = mkError(errInvalidValue, "input sample rate is zero")
		return
	}

	// nframe := uint64(inSize) / ifsz
	nframe := (uint64(inSize) + ifsz - 1) / ifsz
	nframe = (nframe*orate + irate - 1) / irate

	outSize = int(nframe * ofsz)
	return
}

// conv converts audio data from the input format to the output format.
func (c *Converter) conv(out []byte, in []byte) (n int) {

	ifmt, ofmt := c.ifmt, c.ofmt

	todo := len(in) / ifmt.frameSize

	for {
		nframe := min(todo, MaxFrame)
		todo -= nframe
		if nframe == 0 && len(in) > 0 {
			break
		}

		// Convert input to internal format
		c.iconv(c.in, in, ifmt, nframe)

		nchani, nchano := ifmt.channelCount, ofmt.channelCount

		// Downmix if input has more channels than output
		if nchani > nchano {
			inbits := (ifmt.bitDepth + 7) / 8

			nch := min(nchani, len(in)/inbits)

			for i := 1; i < nch; i++ {
				c.iconv(c.out, in[inbits*i:], ifmt, nframe)

				mixin(c.in, c.out, nframe)
			}
		}

		// Apply dithering to the input samples before resampling
		c.seed = dither(c.in, ifmt.abits, ofmt.abits, nframe, c.seed)

		// Resample the first channel (mono or left channel)
		nsample := c.chans[0].resample(c.out, c.in, nframe)

		if nsample < 1 {
			// No output samples produced, check if we are done with input
			if nframe == 0 {
				break
			}
		} else {
			// Output samples were produced, proceed with conversion
			c.oconv(out, c.out, ofmt, nsample)
		}

		// Resample and convert the remaining channels
		switch {
		case nchani == nchano:

			iwidth := (ifmt.bitDepth + 7) / 8
			owidth := (ofmt.bitDepth + 7) / 8

			nch := min(nchani, len(in)/iwidth)

			for i := 1; i < nch; i++ {
				c.iconv(c.in, in[iwidth*i:], ifmt, nframe)

				c.seed = dither(c.in, ifmt.abits, ofmt.abits, nframe, c.seed)

				_ = c.chans[i].resample(c.out, c.in, nframe)

				if nsample > 0 {
					// fmt.Printf("Resampling1 channel %d produced %d samples\n", i, nsample)
					c.oconv(out[owidth*i:], c.out, ofmt, nsample)
				}
			}
		case nsample > 0:
			// If the output has more channels than the input, duplicate the first channel
			owidth := (ofmt.bitDepth + 7) / 8

			for i := 1; i < nchano; i++ {
				// fmt.Printf("Resampling2 channel %d produced %d samples\n", i, nsample)
				c.oconv(out[owidth*i:], c.out, ofmt, nsample)
			}
		}

		if nsample > 0 {
			// advance output
			out = out[nsample*ofmt.frameSize:]

			n += nsample * ofmt.frameSize
		}

		if nframe == 0 || todo == 0 {
			// No more input frames to process
			break
		}

		in = in[nframe*ifmt.frameSize:] // advance input
	}

	return n
}

const (
	Nl       = 8       // 2^Nl samples per zero-crossing in fir
	Nη       = 8       // phase bits for filter interpolation
	Np       = Nl + Nη // phase bits (fractional part of fixed-point)
	One      = 1 << Np // unity scale
	Fcopy    = 1
	MaxFrame = 2048
)

type (
	inner func(out []int32, in []byte, format Format, nframe int)
	outer func(out []byte, in []int32, format Format, nframe int)
)

type iconf struct {
	fmt rune
	fun inner
}

var iconvs = []iconf{
	{SignedIntLE, inSLE},
	{SignedIntBE, inSBE},
	{UnsignedIntLE, inULE},
	{UnsignedIntBE, inUBE},
	{Float, inFloat},
	{ALaw, inAlaw},
	{MuLaw, inMulaw},
}

type oconf struct {
	fmt rune
	fun outer
}

var oconvs = []oconf{
	{SignedIntLE, outSLE},
	{SignedIntBE, outSBE},
	{UnsignedIntLE, outULE},
	{UnsignedIntBE, outUBE},
	{Float, outFloat},
}

// signed little-endian
func inSLE(out []int32, in []byte, ifmt Format, nframe int) {
	nbit, framesz := ifmt.bitDepth, ifmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	for i := range nframe {
		var v uint32

		froff := i * framesz

		for j := range splsz {
			v |= uint32(in[froff+j]) << (8 * j)
		}

		out[i] = int32(v << shift)
	}
}

// inSBE converts signed integer samples to int samples, assuming the input is
// signed big-endian
func inSBE(out []int32, in []byte, ifmt Format, nframe int) {
	nbit, framesz := ifmt.bitDepth, ifmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	for i := range nframe {
		var v uint32

		froff := i * framesz

		for j := range splsz {
			v = (v << 8) | uint32(in[froff+j])
		}

		out[i] = int32(v << shift)
	}
}

// inULE converts unsigned integer samples to int samples, assuming the input is
// unsigned little-endian
func inULE(out []int32, in []byte, ifmt Format, nframe int) {
	nbit, framesz := ifmt.bitDepth, ifmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	// bias := uint32(math.MaxInt32)
	bias := uint32(1) << 31

	for i := range nframe {
		off := i * framesz

		var v uint32

		for j := range splsz {
			v |= uint32(in[off+j]) << (8 * j)
		}

		out[i] = int32((v << shift) - bias)
	}
}

// inUBE converts unsigned integer samples to int samples, assuming the input is
// unsigned big-endian
func inUBE(out []int32, in []byte, ifmt Format, nframe int) {
	nbit, nskip := ifmt.bitDepth, ifmt.frameSize

	shift := 32 - nbit
	splsz := (nbit + 7) / 8

	// bias := uint32(math.MaxInt32)
	bias := uint32(1) << 31

	for i := range nframe {
		off := i * nskip

		var v uint32

		for j := range splsz {
			v = (v << 8) | uint32(in[off+j])
		}

		out[i] = int32((v << shift) - bias)
	}
}

func inFloat32(out []int32, in []byte, framesz, nframe int) {
	const scale = float32(1 << 31)

	bnu32 := binary.NativeEndian.Uint32

	for i := range nframe {
		froff := i * framesz

		f32 := math.Float32frombits(bnu32(in[froff:]))

		switch {
		case f32 >= 1:
			out[i] = math.MaxInt32

		case f32 <= -1:
			out[i] = math.MinInt32

		default:
			out[i] = int32(f32 * scale)
		}
	}
}

func inFloat64(out []int32, in []byte, framesz, nframe int) {
	const scale = float64(1 << 31)

	for i := range nframe {
		froff := i * framesz

		f64 := math.Float64frombits(
			binary.NativeEndian.Uint64(in[froff:]),
		)

		switch {
		case f64 >= 1:
			out[i] = math.MaxInt32

		case f64 <= -1:
			out[i] = math.MinInt32

		default:
			out[i] = int32(f64 * scale)
		}
	}
}

// inFloat converts float samples to int samples, assuming the input is either
// 32-bit or 64-bit float samples.
func inFloat(out []int32, in []byte, ifmt Format, nframe int) {
	nbit, framesz := ifmt.bitDepth, ifmt.frameSize

	if nbit == 32 {
		inFloat32(out, in, framesz, nframe)
	} else {
		inFloat64(out, in, framesz, nframe)
	}
}

// inAlaw converts A-law samples to int samples.
func inAlaw(out []int32, in []byte, ifmt Format, nframe int) {
	framesz := ifmt.frameSize

	var a byte

	for i := range nframe {
		froff := i * framesz

		a = in[froff] ^ 0x55

		t := int32(a&0xf) << 4

		seg := int32((a & 0x70) >> 4)

		switch seg {
		case 0:
			t += 8
		case 1:
			t += 0x108
		default:
			t += 0x108
			t <<= seg - 1
		}

		if a&0x80 == 0 {
			t = -t
		}

		out[i] = t << 16
	}
}

// inμ converts μ-law samples to int samples.
func inMulaw(out []int32, in []byte, ifmt Format, nframe int) {
	framesz := ifmt.frameSize

	var μ byte

	for i := range nframe {
		froff := i * framesz

		μ = in[froff]
		μ = ^μ

		t := (int32(μ&0xf) << 3) + 0x84
		t <<= ((μ & 0x70) >> 4)
		t = 0x84 - t

		if μ&0x80 == 0 {
			t = -t
		}

		out[i] = t << 16
	}
}

// outS converts int samples to signed integer samples, assuming the output is
// signed little-endian
func outSLE(out []byte, in []int32, ofmt Format, nframe int) {
	nbit, framesz := ofmt.bitDepth, ofmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	for i := range nframe {
		froff := i * framesz

		v := in[i] >> shift

		for j := range splsz {
			out[froff+j] = byte(v & 0xff)
			v >>= 8
		}
	}
}

// outSBE converts int samples to signed integer samples, assuming the output is
// signed big-endian
func outSBE(out []byte, in []int32, ofmt Format, nframe int) {
	nbit, framesz := ofmt.bitDepth, ofmt.frameSize

	splsz := (nbit + 7) / 8
	s := 32 - nbit

	for i := range nframe {
		froff := i * framesz

		v := in[i] >> s

		for j := splsz - 1; j >= 0; j-- {
			out[froff+j] = byte(v & 0xff)
			v >>= 8
		}
	}
}

// outULE converts int samples to unsigned integer samples, assuming the output is
// unsigned little-endian
func outULE(out []byte, in []int32, ofmt Format, nframe int) {
	nbit, framesz := ofmt.bitDepth, ofmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	// bias := uint32(math.MaxInt32)
	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz

		v := (bias + uint32(in[i])) >> shift

		for j := range splsz {
			out[froff+j] = byte(v & 0xff)
			v >>= 8
		}
	}
}

// outUBE converts int samples to unsigned integer samples, assuming the output is
// unsigned big-endian
func outUBE(out []byte, in []int32, ofmt Format, nframe int) {
	nbit, framesz := ofmt.bitDepth, ofmt.frameSize

	splsz := (nbit + 7) / 8
	shift := 32 - nbit

	// bias := uint32(math.MaxInt32)
	bias := uint32(1) << 31

	for i := range nframe {
		froff := i * framesz

		v := (bias + uint32(in[i])) >> shift

		for j := splsz - 1; j >= 0; j-- {
			out[froff+j] = byte(v & 0xff)
			v >>= 8
		}
	}
}

func outFloat32(out []byte, in []int32, framesz, nframe int) {
	const scale = float32(1 << 31)

	putu32 := binary.NativeEndian.PutUint32

	for i := range nframe {
		froff := i * framesz

		f := float32(in[i]) / scale

		putu32(out[froff:], math.Float32bits(f))
	}
}

// outFloat64 converts int samples to float samples, assuming the output is float64.
func outFloat64(out []byte, in []int32, framesz, nframe int) {
	const scale = float64(1 << 31)

	putu64 := binary.NativeEndian.PutUint64

	for i := range nframe {
		froff := i * framesz

		f := float64(in[i]) / scale

		putu64(out[froff:], math.Float64bits(f))
	}
}

// outFloat converts int samples to float samples, assuming the output is either
// float32 or float64
func outFloat(out []byte, in []int32, ofmt Format, nframe int) {
	nbit, framesz := ofmt.bitDepth, ofmt.frameSize

	if nbit == 32 {
		outFloat32(out, in, framesz, nframe)
	} else {
		outFloat64(out, in, framesz, nframe)
	}
}

// dither adds noise to the input samples
func dither(in []int32, ibits, obits, nframe int, seed uint32) uint32 {
	if ibits >= 32 || obits >= ibits {
		return seed
	}

	for i := range nframe {
		seed = seed*0x19660d + 0x3c6ef35f

		d := int32(seed) >> ibits
		in[i] = clip(int64(in[i]) + int64(d))
	}

	return seed
}

// mixin mixes the input samples into the output samples
func mixin(y []int32, x []int32, nframe int) {
	for i := range nframe {
		y[i] = clip(int64(y[i]) + int64(x[i]))
	}
}

// min returns the minimum of the given integers
func clip(v int64) int32 {
	const (
		maxInt32 = int64(math.MaxInt32)
		minInt32 = int64(math.MinInt32)
	)
	return int32(clamp(v, minInt32, maxInt32))
}

// clamp clamps the input value to the range [a, b]
func clamp[T cmp.Ordered](v, a, b T) T {
	v = max(v, a)
	v = min(v, b)

	return v
}

// has checks if the specified flag is set in the given integer value.
func has[T constraints.Integer](v T, flag T) bool {
	return (v & flag) != 0
}
