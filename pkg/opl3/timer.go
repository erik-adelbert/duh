// duh OPL3 emulator package
//
// Based on Nuked OPL3 by Nuke.YKT.
//
// Original:
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package opl3

import "time"

type timer struct {
	enabled bool
	masked  bool
	rate    time.Duration
	preset  uint8
	counter uint8
	μs      time.Duration
	elapsed bool
}

func newTimer(rate time.Duration) *timer {
	return &timer{
		rate: rate,
	}
}

func (t *timer) enable(state bool) {
	t.enabled = state
}

func (t *timer) mask(state bool) {
	t.masked = state
}

func (t *timer) isElapsed() bool {
	return !t.masked && t.elapsed
}

func (t *timer) clear() {
	t.elapsed = false
}

func (t *timer) reset(preset uint8) {
	if preset > 0 {
		t.preset = preset
	}

	t.counter = 0
	t.μs = 0

	t.clear()
}

func (t *timer) stop() {
	t.enable(false)
}

func (t *timer) tick(μs time.Duration) {
	t.μs += μs

	for t.μs >= t.rate {
		t.μs -= t.rate
		t.count()
	}
}

func (t *timer) count() {
	if !t.enabled {
		return
	}

	switch t.counter {
	case t.preset:
		t.elapsed = true
		t.counter = 0
	default:
		t.counter++
	}
}
