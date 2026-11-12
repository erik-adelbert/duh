// duh ADL midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package main

// mono ring of the most recent samples; analysis uses whatever is stored
type history struct {
	buf  []int16
	n    int // samples stored, up to len(buf)
	head int // next write position
}

func newHistory(size int) *history { return &history{buf: make([]int16, size)} }

func (h *history) reset() { h.n, h.head = 0, 0 }

func (h *history) add(s []int16) {
	// only the newest len(buf) samples can matter
	if len(s) > len(h.buf) {
		s = s[len(s)-len(h.buf):]
	}

	k := copy(h.buf[h.head:], s)
	copy(h.buf, s[k:]) // wrapped remainder, if any

	h.head = (h.head + len(s)) % len(h.buf)
	h.n = min(h.n+len(s), len(h.buf))
}

// window copies the stored samples into dst in chronological order.
func (h *history) window(dst []int16) []int16 {
	start := 0
	if h.n == len(h.buf) {
		start = h.head
	}

	for i := range h.n {
		dst[i] = h.buf[(start+i)%len(h.buf)]
	}

	return dst[:h.n]
}
