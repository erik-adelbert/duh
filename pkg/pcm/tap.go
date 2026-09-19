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
	*Ring

	sync.WaitGroup

	win int
	hop int

	updateFn func(p []byte)

	stop chan struct{}

	startOnce, stopOnce sync.Once
}

func NewTap(format Format, bufferDuration time.Duration, windowSize int, hop int) (*Tap, error) {
	if windowSize <= 0 {
		return nil, mkError(ErrTap, "invalid window size")
	}
	if hop <= 0 || hop > windowSize {
		return nil, mkError(ErrTap, "invalid hop size")
	}

	ring, err := NewRing(format, bufferDuration)
	if err != nil {
		return nil, err
	}

	return &Tap{
		Ring: ring,

		win: windowSize,
		hop: hop,

		stop: make(chan struct{}),
	}, nil
}

func (tap *Tap) Write(p []byte) (n int, err error) {
	select {
	case <-tap.stop:
		return len(p), nil
	default:
	}

	n, err = tap.Ring.Write(p)

	return
}

func (tap *Tap) WindowSize() int {
	return tap.win
}

func (tap *Tap) Start() {
	tap.startOnce.Do(func() {
		tap.Go(func() {
			fs := int(tap.FrameSize())
			chunk := make([]byte, tap.win*fs)

			// The display advances by hop frames per update.
			hop := time.Duration(
				float64(tap.hop) /
					float64(tap.SampleRate()) *
					float64(time.Second),
			)

			updater := time.NewTicker(hop)
			defer updater.Stop()

			for {
				select {
				case <-tap.stop:
					return

				case <-updater.C:
					if tap.ReadWindow(chunk, tap.hop) {
						if tap.updateFn != nil {
							tap.updateFn(chunk)
						}
					}
				}
			}
		})
	})
}

// UpdateFunc registers the callback invoked by Start with the latest decoded
// window. It must be called before Start.
// The samples slice is only valid for the duration of the callback and
// must not be retained or modified after the callback returns.
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
	tap.stopOnce.Do(func() {
		close(tap.stop)
	})
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
	if r.r == nil {
		return 0, nil
	}

	n, err = r.r.Read(p)

	if n > 0 {
		_, _ = r.Write(p[:n])
	}

	return n, err
}

func (r *TapReader) Seek(offset int64, whence int) (int64, error) {
	if r.rs == nil {
		return 0, ErrESPIPE
	}

	pos, err := r.rs.Seek(offset, whence)
	if err != nil {
		err = mkError(ErrTap, err)
		return 0, err
	}

	r.Reset()

	return pos, nil
}
