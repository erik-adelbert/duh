// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"io"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/erik-adelbert/duh/pkg/pcm"
)

type OtoBackend struct {
	ctx *oto.Context
	o   *oto.NewContextOptions

	r        *otoReader
	p        *oto.Player
	duration time.Duration

	posLast time.Time

	ofmt pcm.Format

	flush func() int
}

func isOtoFormat(format pcm.Format) bool {
	var otofmts = []string{
		"u8", "s16", "f32",
	}

	fstr := format.String()

	nchan := format.ChannelCount()
	sr := format.SampleRate()

	for f := range otofmts {
		if strings.HasPrefix(fstr, otofmts[f]) {
			if nchan <= 2 && (sr == 44_100 || sr == 48_000) {
				return true
			}
		}
	}

	return false
}

func NewOtoBackend(r io.Reader, format pcm.Format, wasm bool) (*OtoBackend, error) {
	ofmt := format

	if ofmt.Tag() == pcm.UnsignedIntBE && ofmt.BitDepth() == 8 {
		ofmt, _ = pcm.NewFormat(
			pcm.UnsignedIntLE, // 8-bit big-endian is little-endian too
			ofmt.BitDepth(), ofmt.ChannelCount(), ofmt.SampleRate(),
		)
	}

	var flush = func() int { return 0 }

	if !isOtoFormat(ofmt) {
		ofmt32, err := pcm.NewFormat(pcm.Float, 32, 2, 44_100)
		if err != nil {
			return nil, err
		}

		cr, err := pcm.NewConverter(ofmt32, ofmt)
		if err != nil {
			return nil, err
		}

		flush = func() int { return cr.Flush(nil) }

		r = pcm.OpenConv(r, cr)
		ofmt = ofmt32
	}

	var otofmt oto.Format

	switch ofmt.BitDepth() {
	case 8:
		otofmt = oto.FormatUnsignedInt8
	case 16:
		otofmt = oto.FormatSignedInt16LE
	case 32:
		otofmt = oto.FormatFloat32LE
	}

	opts := oto.NewContextOptions{
		Format:       otofmt,
		ChannelCount: ofmt.ChannelCount(),
		SampleRate:   ofmt.SampleRate(),
	}

	ctx, ready, err := oto.NewContext(&opts)

	if err != nil {
		return nil, err
	}

	if wasm { // don't block
		go func() {
			<-ready
		}()
	} else {
		<-ready
	}

	or := &otoReader{
		r:      r,
		format: opts.Format,
		count:  0,
	}

	return &OtoBackend{
		ctx: ctx,
		o:   &opts,
		r:   or,
		p:   ctx.NewPlayer(or),

		ofmt:  ofmt,
		flush: flush,
	}, nil
}

func (b *OtoBackend) Close() {
	if b.flush != nil {
		_ = b.flush()
	}
}

func (b *OtoBackend) Format() pcm.Format {
	return b.ofmt
}

func (b *OtoBackend) BitDepth() int {
	switch b.o.Format {
	case oto.FormatUnsignedInt8:
		return 8
	case oto.FormatSignedInt16LE:
		return 16
	case oto.FormatFloat32LE:
		return 32
	default:
		return 0
	}
}

func (b *OtoBackend) ChannelCount() int {
	return b.o.ChannelCount
}

func (b *OtoBackend) Duration() (time.Duration, error) {
	rs, ok := b.r.r.(io.Seeker)

	if !ok {
		return 0, ErrESPIPE
	}

	if b.p == nil {
		return 0, nil
	}

	if b.duration != 0 {
		return b.duration, nil
	}

	// Save current position
	cur, _ := rs.Seek(0, io.SeekCurrent)
	end, err := rs.Seek(0, io.SeekEnd)

	if err != nil {
		return 0, err
	}

	// Restore position
	_, _ = rs.Seek(cur, io.SeekStart)

	var fsz int64

	switch b.o.Format {
	case oto.FormatUnsignedInt8:
		fsz = int64(b.o.ChannelCount * 1)
	case oto.FormatSignedInt16LE:
		fsz = int64(b.o.ChannelCount * 2)
	case oto.FormatFloat32LE:
		fsz = int64(b.o.ChannelCount * 4)
	default:
		return 0, ErrBadFormat
	}

	nsample := end / fsz
	b.duration = time.Duration(int64(time.Second) * nsample / int64(b.o.SampleRate))

	return b.duration, nil
}

func (b *OtoBackend) IsPlaying() bool {
	return b.p != nil && b.p.IsPlaying()
}

func (b *OtoBackend) Pause() {
	if b.p != nil {
		b.p.Pause()
	}
}

func (b *OtoBackend) Play() {
	if b.p != nil {
		b.posLast = time.Now()

		b.p.Play()
	}
}

func (b *OtoBackend) Position() int64 {
	now := time.Now()
	dt := now.Sub(b.posLast)

	b.posLast = now
	bytesPerSecond := int64(b.o.SampleRate * b.o.ChannelCount * (b.BitDepth() / 8))
	advance := bytesPerSecond * dt.Nanoseconds() / int64(time.Second)

	return max(b.r.Count()-advance, 0)
}

func (b *OtoBackend) SampleRate() int {
	return b.o.SampleRate
}

func (b *OtoBackend) Seek(offset int64, whence int) (int64, error) {
	if b.r == nil {
		return 0, ErrInvalidSeek
	}

	return b.p.Seek(offset, whence)
}

func (b *OtoBackend) SetVolume(volume float64) {
	if b.p != nil {
		b.p.SetVolume(volume)
	}
}

type otoReader struct {
	r      io.Reader
	format oto.Format
	count  int64
}

func (or *otoReader) Count() int64 {
	return or.count
}

func (or *otoReader) Read(p []byte) (int, error) {
	n, err := or.r.Read(p)

	or.count += int64(n)

	return n, err
}

func (or *otoReader) Seek(offset int64, whence int) (int64, error) {
	rs, ok := or.r.(io.Seeker)

	if !ok {
		return 0, ErrESPIPE
	}

	pos, err := rs.Seek(offset, whence)

	if err != nil {
		return 0, err
	}

	or.count = pos

	return pos, nil
}
