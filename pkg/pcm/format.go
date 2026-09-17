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
	"fmt"
	"math"
	"strings"
	"time"
)

// Format describes the format of PCM audio data.
type Format struct {
	sampleRate   int
	channelCount int
	bitDepth     int // bits in input stream per sample

	frameSize int
	abits     int // bits after input conversion
	tag       Tag
}

// Common PCM format strings
const (
	SLE16Mono8k = "s16c1r8000"

	SLE16Mono44k = "s16c1r44100"
	SLE16Mono48k = "s16c1r48000"

	SLE16Stereo44k = "s16c2r44100"

	SLE16Stereo48k = "s16c2r48000"

	U8Mono8k    = "u8c1r8000"
	U8Stereo44k = "u8c2r44100"

	AudioCD = SLE16Stereo44k
	OPL3    = SLE16Stereo44k
)

// DefaultFormat returns the default PCM format s16c2r44100,
// which is 16-bit signed, 2-channel, 44100 Hz.
func DefaultFormat() Format {
	f, _ := ParseFormat(SLE16Stereo44k)
	return f
}

// NewFormat creates a new PCM Format with the specified parameters.
func NewFormat(tag rune, bitDepth, channelCount, sampleRate int) (f Format, err error) {
	// On all exit paths, if there was an error, wrap it with context
	// and return a zeroed PCMFormat.
	defer func() {
		err = mkError(ErrFormat, err)
	}()

	bad := func(s string) error {
		return mkError(errInvalidValue, s)
	}

	if tag == UnsignedIntBE && bitDepth == 8 {
		tag = UnsignedIntLE // 8-bit big-endian is little-endian too
	}

	abits := bitDepth

	switch {
	case sampleRate <= 0, channelCount <= 0:
		err = bad("rate and channel count must be positive")

	case tag == ALaw || tag == MuLaw:
		if bitDepth != 8 {
			err = bad("μ-law and A-law must be 8 bits")
		}
		abits = 16

	case tag == Float:
		if bitDepth != 32 && bitDepth != 64 {
			err = bad("float must be 32 or 64 bits")
		}
		abits = bitDepth

	case bitDepth <= 0 || bitDepth > 32:
		err = bad("unsupported bit depth")
	}

	if err != nil {
		return
	}

	splsz := (bitDepth + 7) / 8
	framesz := splsz * channelCount

	if framesz <= 0 {
		err = bad("frame size must be positive")
		return
	}

	f = Format{
		sampleRate:   sampleRate,
		channelCount: channelCount,
		bitDepth:     bitDepth,
		abits:        abits,
		tag:          tag,
		frameSize:    framesz,
	}

	return
}

// ParseFormat parses a PCM format string and returns a Format.
// The format string should be in the form of "s16c2r44100" for 16-bit
// signed, 2-channel, 44100 Hz PCM data.
func ParseFormat(s string) (f Format, err error) {
	// On all exit paths, if there was an error, wrap it with context
	// and return a zeroed PCMFormat.
	defer func() {
		err = mkError(ErrFormat, err)
	}()

	if s == "" {
		err = mkError(errBadFormat, "empty format")

		return
	}

	var (
		tag          rune
		channelCount int
		sampleRate   int
		bits         int
	)

	sr := strings.NewReader(s)

	var field rune

	for sr.Len() > 0 {
		field, _, err = sr.ReadRune()

		switch field {
		case 0:
			// do nothing more
		case Rate:
			_, err = fmt.Fscanf(sr, "%d", &sampleRate)
		case ChannelCount:
			_, err = fmt.Fscanf(sr, "%d", &channelCount)
		case 'm':
			field = 'μ'
			fallthrough
		case ALaw, MuLaw, Float,
			SignedIntLE, SignedIntBE,
			UnsignedIntLE, UnsignedIntBE:

			tag = field

			_, err = fmt.Fscanf(sr, "%d", &bits)
		default:
			err = mkErrorf(errInvalidField, "unknown format character %c", field)
		}

		if err != nil {
			return
		}
	}

	return NewFormat(tag, bits, channelCount, sampleRate)
}

func (f Format) String() string {
	return fmt.Sprintf("%c%dc%dr%d", f.tag, f.bitDepth, f.channelCount, f.sampleRate)
}

func (f Format) SampleRate() int {
	return int(f.sampleRate)
}

func (f Format) ChannelCount() int {
	return int(f.channelCount)
}

func (f Format) FrameSize() int64 {
	return int64(f.frameSize)
}

func (f Format) SampleSize() int64 {
	return int64(f.frameSize / f.channelCount)
}

func (f Format) Align(size int) int {
	return (size / f.frameSize) * f.frameSize
}

func (f Format) BitDepth() int {
	return int(f.bitDepth)
}

func (f Format) Tag() rune {
	return f.tag
}

func (f Format) Duration(size int64) time.Duration {
	ch, bd := int64(f.channelCount), int64(f.bitDepth)

	nsample := size / ch / (bd / 8)

	splf64 := float64(nsample)
	srf64 := float64(f.sampleRate)

	dt := time.Duration(splf64 / srf64 * 1e9)

	return dt
}

func (f Format) FrameCount(dt time.Duration) int64 {
	return int64(dt) * int64(f.sampleRate) / int64(time.Second)
}

func (f Format) SampleCount(dt time.Duration) int64 {
	return f.FrameCount(dt) * int64(f.channelCount)
}

func (f Format) BufferSize(duration time.Duration) int64 {
	const DefaultDuration = 200 * ms

	if duration <= 0 {
		duration = DefaultDuration
	}

	return f.SampleCount(duration) * int64(f.frameSize)
}

type Tag = rune

// PCM format tags
const (
	ALaw          Tag = 'a'
	Float         Tag = 'f'
	ChannelCount  Tag = 'c'
	MuLaw         Tag = 'μ'
	Rate          Tag = 'r'
	SignedIntLE   Tag = 's'
	SignedIntBE   Tag = 'S'
	UnsignedIntLE Tag = 'u'
	UnsignedIntBE Tag = 'U'
)

func (f Format) EncodeF64(out []byte, in64 []float64, scratch32 []int32) (nsample int) {
	nchan := f.channelCount

	nframe := min(
		len(out)/f.frameSize,
		len(in64)/nchan,
		len(scratch32)/nchan,
	)

	nsample = nframe * nchan

	for i := range nsample {
		scratch32[i] = int32(in64[i] * float64(math.MaxInt32))
	}

	encode(out, scratch32[:nsample], f, nframe)

	return
}

// DecodeF64 decodes PCM data from the specified format into floating-point samples.
func (f Format) DecodeF64(out64 []float64, in []byte, scratch32 []int32) (nsample int) {
	nchan := f.channelCount

	nframe := min(
		len(in)/f.frameSize,
		len(out64)/nchan,
		len(scratch32)/nchan,
	)

	if nframe == 0 {
		return
	}

	nsample = nframe * nchan

	bias := float64(math.MaxInt32)

	decode(scratch32[:nsample], in, f, nframe)

	for i, s32 := range scratch32[:nsample] {
		out64[i] = float64(s32) / bias
	}

	return
}

func (f Format) DecodeS16(out16 []int16, in []byte, scratch32 []int32) (nsample int) {
	if f.frameSize <= 0 || f.channelCount <= 0 {
		return
	}

	nframe := min(
		len(in)/f.frameSize,
		len(out16)/f.channelCount,
		len(scratch32),
	)

	if nframe == 0 {
		return
	}

	nsample = nframe * f.channelCount

	decode(scratch32[:nsample], in, f, nframe)

	for i, s32 := range scratch32[:nsample] {
		out16[i] = int16(s32 >> 16)
	}

	return
}

const ms = time.Millisecond
