// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"fmt"
	"io"
	"time"

	"github.com/erik-adelbert/duh/pkg/backend"
)

type Controller struct {
	p backend.Player
}

func NewController(p backend.Player) *Controller {
	return new(Controller{p: p})
}

func (c *Controller) IsPlaying() bool {
	return c.p.IsPlaying()
}

func (c *Controller) Play() {
	if c.hasEnded() {
		c.Rewind()
	}

	c.p.Play()
}

func (c *Controller) Pause() {
	c.p.Pause()
}

func (c *Controller) Toggle() {
	if c.IsPlaying() {
		c.Pause()

		return
	}

	c.Play()
}

func (c *Controller) SetVolume(volume float64) {
	c.p.SetVolume(volume)
}

func (c *Controller) Snapshot() (Position, Position, bool) {
	playing := c.IsPlaying()
	pos, dur := c.position()

	return pos, dur, playing
}

func (c *Controller) hasEnded() bool {
	pos, dur := c.position()

	return !c.p.IsPlaying() && abs(pos-dur) <= Position(500*time.Millisecond)
}

func (c *Controller) position() (Position, Position) {
	off := c.p.Position()
	spr := c.p.SampleRate()
	nch := c.p.ChannelCount()
	bps := c.p.BitDepth() / 8

	pos := int64(time.Second) * off / int64(spr*nch*bps)

	dur := c.p.Duration()

	return Position(pos), Position(dur)
}

func (c *Controller) offset(p Position) int64 {
	spr := c.p.SampleRate()
	nch := c.p.ChannelCount()
	bps := c.p.BitDepth() / 8

	return int64(p) * int64(spr*nch*bps) / int64(time.Second)
}

func (c *Controller) Rewind() {
	_, _ = c.p.Seek(0, io.SeekStart)
}

func (c *Controller) ForwardBy(d Position) {
	pos, dur := c.position()

	if pos+d > dur {
		d = dur - pos
	}

	dst := c.offset(d + pos)
	_, _ = c.p.Seek(dst, io.SeekStart)
}

func (c *Controller) RewindBy(d Position) {
	pos, _ := c.position()

	if pos < d {
		d = pos
	}

	dst := c.offset(pos - d)
	_, _ = c.p.Seek(dst, io.SeekStart)
}

type Position time.Duration

func (p Position) String() string {
	d := time.Duration(p)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60

	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}

	return fmt.Sprintf("%02d:%02d", m, s)
}

type (
	MeterValues   = backend.MeterValues
	SpectrumValue = backend.SpectrumValue
)

func abs[T ~int64 | ~int](a T) T {
	if a < 0 {
		return -a
	}

	return a
}
