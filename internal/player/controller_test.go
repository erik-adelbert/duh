// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import (
	"errors"
	"math"
	"testing"
	"time"

	offcalc "github.com/erik-adelbert/duh/pkg/offset"
)

type testPlayer struct {
	pos int64
	err error
}

func (*testPlayer) BitDepth() int                  { return 16 }
func (*testPlayer) ChannelCount() int              { return 2 }
func (*testPlayer) SampleRate() int                { return 44100 }
func (*testPlayer) Duration() time.Duration        { return time.Second }
func (*testPlayer) IsPlaying() bool                { return true }
func (*testPlayer) Pause()                         {}
func (*testPlayer) Play()                          {}
func (*testPlayer) SetVolume(float64)              {}
func (p *testPlayer) Position() (int64, error)     { return p.pos, p.err }
func (*testPlayer) Seek(int64, int) (int64, error) { return 0, nil }

func TestControllerPositionErrors(t *testing.T) {
	failure := errors.New("position failed")
	c := NewController(&testPlayer{err: failure})
	if _, _, _, err := c.Snapshot(); !errors.Is(err, failure) {
		t.Fatalf("Snapshot error = %v", err)
	}
	if err := c.ForwardBy(Position(time.Second)); !errors.Is(err, failure) {
		t.Fatalf("ForwardBy error = %v", err)
	}
	c = NewController(&testPlayer{pos: math.MaxInt64})
	if _, _, _, err := c.Snapshot(); !errors.Is(err, offcalc.ErrOverflow) {
		t.Fatalf("overflow error = %v", err)
	}
}
