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
	*oto.Context

	pcm.Format

	ofmt pcm.Format
	opts *oto.NewContextOptions
}

func NewOtoBackend(format pcm.Format, wasm bool) (ob *OtoBackend, err error) {
	defer func() {
		err = mkError(ErrBackend, err)
	}()

	ifmt := format
	ofmt := format

	switch {
	case ifmt.Tag() == pcm.UnsignedIntBE && ifmt.BitDepth() == 8:
		ifmt, _ = pcm.NewFormat(
			pcm.UnsignedIntLE, // 8-bit big-endian is little-endian too
			ifmt.BitDepth(), ifmt.ChannelCount(), ifmt.SampleRate(),
		)

		ofmt = ifmt
	case !isOtoFormat(ifmt):
		nchan := clamp(ifmt.ChannelCount(), 1, 2)

		ofmt, _ = pcm.NewFormat(
			pcm.Float, 32, nchan, ifmt.SampleRate(),
		)
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

	return &OtoBackend{
		Context: ctx,
		Format:  ifmt,

		opts: &opts,
		ofmt: ofmt,
	}, nil
}

type OtoPlayer struct {
	*oto.Player
	pcm.Format

	r  io.Reader
	rs io.ReadSeeker
}

func (b *OtoBackend) NewPlayer(r io.Reader) *OtoPlayer {
	if r == nil {
		return nil
	}

	rs, _ := r.(io.ReadSeeker)

	if b.Format != b.ofmt {
		cv, err := pcm.NewConverter(b.Format, b.ofmt)

		if err != nil {
			return nil
		}

		rcv := pcm.OpenConv(r, cv)

		r = rcv

		if rs != nil {
			rs = rcv
		}
	}

	return &OtoPlayer{
		Player: b.Context.NewPlayer(r),
		Format: b.Format,

		r:  r,
		rs: rs,
	}
}

func (b *OtoBackend) Close() {}

func (p *OtoPlayer) Seek(offset int64, whence int) (n int64, err error) {
	defer func() {
		err = mkError(ErrSeek, err)
	}()

	if p.rs == nil {
		return 0, ErrESPIPE
	}

	return p.rs.Seek(offset, whence)
}

func (p *OtoPlayer) Duration() (dt time.Duration) {
	if p.rs == nil {
		return
	}

	off0, err := p.rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}

	end, err := p.rs.Seek(0, io.SeekEnd)
	if err != nil {
		return
	}

	_, err = p.rs.Seek(off0, io.SeekStart)
	if err != nil {
		return
	}

	return p.Format.Duration(end)
}

func (p *OtoPlayer) Position() int64 {
	if p.rs == nil {
		return 0
	}

	off, err := p.rs.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}

	return off
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

func clamp(x, a, b int) int {
	return max(a, min(b, x))
}
