// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
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

	ofmt pcm.Format

	tracker  *tracker
	playback positioner
	mu       sync.Mutex
	duration time.Duration
}

// NewPlayer creates a player and captures seekable stream metadata
// before playback begins. Non-seekable streams have an unknown duration.
func (b *OtoBackend) NewPlayer(r io.Reader) (*OtoPlayer, error) {
	if r == nil {
		return nil, fmt.Errorf("nil audio reader")
	}

	start, duration, seekable, err := inspectReader(r, b.Format)
	if err != nil {
		return nil, err
	}

	r, rs, err := b.prepReader(r)
	if err != nil {
		return nil, err
	}

	if !seekable {
		rs = nil
	}

	tracker := &tracker{reader: r, seeker: rs}
	tracker.offset.Store(remap(start, b.FrameSize(), b.ofmt.FrameSize()))
	player := b.Context.NewPlayer(tracker)
	return &OtoPlayer{
		Player:   player,
		Format:   b.Format,
		ofmt:     b.ofmt,
		tracker:  tracker,
		playback: player,
		duration: duration,
	}, nil
}

func inspectReader(r io.Reader, format pcm.Format) (int64, time.Duration, bool, error) {
	rs, ok := r.(io.ReadSeeker)
	if !ok {
		return 0, 0, false, nil
	}
	var start, end int64
	start, err := rs.Seek(0, io.SeekCurrent)
	switch {
	case errors.Is(err, pcm.ErrESPIPE), errors.Is(err, syscall.ESPIPE):
		return 0, 0, false, nil
	case err != nil:
		return 0, 0, false, fmt.Errorf("get initial audio position: %w", err)
	}
	end, err = rs.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, false, fmt.Errorf("get audio duration: %w", err)
	}
	if _, err = rs.Seek(start, io.SeekStart); err != nil {
		return 0, 0, false, fmt.Errorf("restore initial audio position: %w", err)
	}
	return start, format.Duration(end), true, nil
}

func (b *OtoBackend) prepReader(r io.Reader) (io.Reader, io.ReadSeeker, error) {
	rs, _ := r.(io.ReadSeeker)

	if b.Format != b.ofmt {
		cv, err := pcm.NewConverter(b.ofmt, b.Format)

		if err != nil {
			return nil, nil, err
		}

		rcv := pcm.OpenConv(r, cv)

		r = rcv

		if rs != nil {
			rs = rcv
		}
	}

	return r, rs, nil
}

func (b *OtoBackend) Close() {}

type positioner interface {
	Seek(int64, int) (int64, error)
	BufferedSize() int
}

type tracker struct {
	reader io.Reader
	seeker io.Seeker
	offset atomic.Int64
}

func (r *tracker) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.offset.Add(int64(n))
	return n, err
}

func (r *tracker) Seek(offset int64, whence int) (int64, error) {
	if r.seeker == nil {
		return 0, ErrESPIPE
	}
	pos, err := r.seeker.Seek(offset, whence)
	if err == nil {
		r.offset.Store(pos)
	}
	return pos, err
}

func remap(offset, from, to int64) int64 {
	return (offset/from)*to + (offset%from)*to/from
}

func (p *OtoPlayer) Seek(offset int64, whence int) (n int64, err error) {
	defer func() {
		err = mkError(ErrSeek, err)
	}()

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tracker.seeker == nil {
		return 0, ErrESPIPE
	}

	if whence == io.SeekCurrent {
		offset += p.pos()
		whence = io.SeekStart
	}
	offset = remap(offset, p.FrameSize(), p.ofmt.FrameSize())
	pos, err := p.playback.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	return remap(pos, p.ofmt.FrameSize(), p.FrameSize()), nil
}

func (p *OtoPlayer) Duration() time.Duration { return p.duration }

// Position estimates bytes played in the input format, excluding Oto's buffer.
// Concurrent reads and system audio buffering make the result approximate.
func (p *OtoPlayer) Position() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pos()
}

func (p *OtoPlayer) pos() int64 {
	off := max(p.tracker.offset.Load()-int64(p.playback.BufferedSize()), 0)
	return remap(off, p.ofmt.FrameSize(), p.FrameSize())
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
