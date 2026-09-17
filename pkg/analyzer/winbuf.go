// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

// wbuf is a generic sliding window buffer that supports
// writing data and reading fixed-size windows.
type wbuf[T any] struct {
	buf []T
	cap int

	winsz int

	hopsz int

	wpos int
	wcnt int

	rpos int

	pending int
	filled  int
}

func newbuf[T any](cap, winSize, hopSize int) (wb *wbuf[T], err error) {
	if winSize > cap {
		err = mkError(ErrNew, "window size cannot be larger than buffer capacity")
		return
	}

	if hopSize <= 0 || hopSize > winSize {
		err = mkError(ErrNew, "hop size must be between 1 and window size")
		return
	}

	wb = &wbuf[T]{
		buf:   make([]T, cap),
		cap:   cap,
		winsz: winSize,
		hopsz: hopSize,
	}

	return
}

func (r *wbuf[T]) write(values ...T) int {
	nwrite := 0

	for _, v := range values {

		r.buf[r.wpos] = v
		r.wpos = (r.wpos + 1) % r.cap
		r.wcnt++

		if r.filled < r.cap {
			r.filled++
		}

		if r.pending < r.cap {
			r.pending++
		}

		// r.pending += r.filled
		nwrite++
	}

	return nwrite
}

func (r *wbuf[T]) readWindow(p []T) (n int, err error) {
	if r.pending < r.winsz {
		// Not enough data available to read a full window.
		return
	}

	if len(p) < r.winsz {
		return 0, mkError(ErrRead, "destination slice too small for window")
	}

	oldAbs := r.wcnt - r.filled

	bufStart := 0
	if r.filled == r.cap {
		bufStart = r.wpos
	}

	off := r.rpos - oldAbs
	start := (bufStart + off) % r.cap

	// Copy the window data into p, handling wrap-around if necessary.
	todo := r.winsz

	i := 0
	for todo > 0 {
		chunk := r.cap - start
		chunk = min(todo, chunk)

		copy(p[i:i+chunk], r.buf[start:start+chunk])

		if todo -= chunk; todo > 0 {
			start = 0
		}

		i += chunk
	}

	n = r.winsz
	r.rpos += r.hopsz
	r.pending -= r.hopsz

	return
}

func (r *wbuf[T]) available() int {
	return r.pending
}
