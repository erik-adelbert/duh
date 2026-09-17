// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcm

import (
	"io"
	"sync"
	"time"
)

type Tap struct {
	Format
	*Ring

	sync.WaitGroup

	win int

	updateFn func(p []byte)
	tick     time.Duration // Update tick

	stop chan struct{}
}

const DefaultTapCallBackRate = 30 * FPS

func NewTap(format Format, bufferDuration time.Duration, windowSize int, tick time.Duration) (*Tap, error) {
	if tick <= 0 {
		tick = time.Second / DefaultTapCallBackRate
	}

	ring, err := NewRing(format, bufferDuration)
	if err != nil {
		return nil, err
	}

	return &Tap{
		Format: format,
		Ring:   ring,

		win: windowSize,

		tick: tick,

		stop: make(chan struct{}),
	}, nil
}

func (tap *Tap) Write(p []byte) (n int, err error) {
	select {
	case <-tap.stop:
		return
	default:
		n, err = tap.Ring.Write(p)
	}

	return
}

func (tap *Tap) WindowSize() int {
	return tap.win
}

func (tap *Tap) Start() {
	tap.Go(func() {
		fs := int(tap.FrameSize())
		chunk := make([]byte, tap.win*fs)

		clock := time.NewTicker(tap.tick)
		defer clock.Stop()

		for {
			select {
			case <-tap.stop:
				return
			case <-clock.C:
				nread := tap.ReadFrames(chunk)

				if nread > 0 && tap.updateFn != nil {
					tap.updateFn(chunk[:nread])
				}
			}
		}
	})
}

func (tap *Tap) UpdateFunc(update func([]float64, int)) {
	nchan := tap.ChannelCount()
	nframe := tap.WindowSize()

	samples := make([]float64, nframe*nchan)
	scratch32 := make([]int32, nframe*nchan)

	tap.updateFn = func(p []byte) {
		nsample := tap.DecodeF64(samples, p, scratch32)

		update(samples[:nsample], nchan)
	}
}

// Stop everything
func (tap *Tap) Stop() {
	close(tap.stop)
	tap.Wait()
}

type TapReader struct {
	r  io.Reader
	rs io.ReadSeeker

	*Tap
}

func OpenTap(r io.Reader, tap *Tap) *TapReader {
	var rs io.ReadSeeker

	_, ok := r.(io.ReadSeeker)

	if ok {
		rs = r.(io.ReadSeeker)
	}

	return &TapReader{
		r:   r,
		rs:  rs,
		Tap: tap,
	}
}

func (r *TapReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)

	if err != nil {
		return
	}

	if n > 0 && r.Tap != nil {
		select {
		case <-r.stop:
			return
		default:
			n, err = r.Write(p[:n])

			if err != nil {
				return
			}
		}
	}

	return
}

func (r *TapReader) Seek(offset int64, whence int) (int64, error) {
	if r.rs != nil {
		return r.rs.Seek(offset, whence)
	}

	return 0, ErrESPIPE
}

const FPS = 1
