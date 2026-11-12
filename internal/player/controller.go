// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"fmt"
	"io"
	"time"

	offcalc "github.com/erik-adelbert/duh/pkg/offset"
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

func (c *Controller) Play() error {
	ended, err := c.hasEnded()
	if err != nil {
		return err
	}
	if ended {
		if err := c.Rewind(); err != nil {
			return err
		}
	}

	c.p.Play()
	return nil
}

func (c *Controller) Pause() {
	c.p.Pause()
}

func (c *Controller) Toggle() error {
	if c.IsPlaying() {
		c.Pause()
		return nil
	}

	return c.Play()
}

func (c *Controller) SetVolume(volume float64) {
	c.p.SetVolume(volume)
}

func (c *Controller) Snapshot() (Position, Position, bool, error) {
	playing := c.IsPlaying()
	pos, dur, err := c.position()

	return pos, dur, playing, err
}

func (c *Controller) hasEnded() (bool, error) {
	pos, dur, err := c.position()
	if err != nil {
		return false, err
	}

	return !c.p.IsPlaying() && abs(pos-dur) <= Position(500*time.Millisecond), nil
}

func (c *Controller) position() (Position, Position, error) {
	off, err := c.p.Position()
	if err != nil {
		return 0, 0, fmt.Errorf("playback position: %w", err)
	}
	spr := c.p.SampleRate()
	nch := c.p.ChannelCount()
	bps := c.p.BitDepth() / 8

	pos, err := offcalc.Scale(off, int64(spr)*int64(nch)*int64(bps), int64(time.Second))
	if err != nil {
		return 0, 0, fmt.Errorf("playback time: %w", err)
	}

	dur := c.p.Duration()

	return Position(pos), Position(dur), nil
}

func (c *Controller) offset(p Position) (int64, error) {
	spr := c.p.SampleRate()
	nch := c.p.ChannelCount()
	bps := c.p.BitDepth() / 8

	return offcalc.Scale(int64(p), int64(time.Second), int64(spr)*int64(nch)*int64(bps))
}

func (c *Controller) Rewind() error {
	_, err := c.p.Seek(0, io.SeekStart)
	return err
}

func (c *Controller) ForwardBy(d Position) error {
	pos, dur, err := c.position()
	if err != nil {
		return err
	}

	if d > dur-pos {
		d = dur - pos
	}

	dst, err := c.offset(d + pos)
	if err != nil {
		return err
	}
	_, err = c.p.Seek(dst, io.SeekStart)
	return err
}

func (c *Controller) RewindBy(d Position) error {
	pos, _, err := c.position()
	if err != nil {
		return err
	}

	if pos < d {
		d = pos
	}

	dst, err := c.offset(pos - d)
	if err != nil {
		return err
	}
	_, err = c.p.Seek(dst, io.SeekStart)
	return err
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
