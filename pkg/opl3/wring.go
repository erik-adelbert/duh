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

const (
	BufferSize  = 1024
	BufferDelay = 2
)

// wring is a write ring buffer for the OPL3 chip. It buffers writes to the chip and
// ensures that they are written in the correct order and at the correct time.
type wring struct {
	buf [BufferSize]wrop
	bufState

	write func(uint16, uint8)
}

// bufState holds the state of the write ring buffer.
type bufState struct {
	nwrite    uint64 // number of writes to the buffer 1 write per sample
	t0        uint64
	cur, last uint32
}

// wrop represents a write to the OPL3 chip. It contains the time at which
// the write should be performed, the register to write to, and the data to write.
type wrop struct {
	time uint64 // time in samples
	reg  uint16
	data uint8
}

// bit 9 is not used in the chip register space, so we can use it to mark a write as
// ready to be written to the chip.
const ready = 1 << 9

// push adds a write to the ring buffer
func (wr *wring) push(reg uint16, data uint8) {
	state := &wr.bufState

	// Insert the write into the buffer at the last position.
	last := wr.last

	w := &wr.buf[last]

	// If the last write is already ready, write it to the chip before
	// overwriting it.
	if has(w.reg, ready) {
		w.reg &^= ready

		wr.write(w.reg, w.data)

		state.cur = (last + 1) % BufferSize
		state.nwrite = w.time
	}

	// Fill in the write at the last position.
	w.reg = reg | ready
	w.data = data

	// If the write happens in the past, we need to advance the time to the
	// current sample count plus a delay to ensure that the write happens at
	// least at the current time.

	now := wr.nwrite // current sample count

	t0 := wr.t0 + BufferDelay // desired time for the next write

	t0 = max(t0, now) // t0 is at least the current time

	w.time = t0

	// Update the time for the next write and advance the last index.
	state.t0 = t0
	state.last = (last + 1) % BufferSize
}

// flush writes all pending writes in the ring buffer to the chip.
func (wr *wring) flush() {

	for {
		w := &wr.buf[wr.cur]

		// Stop if the write is not ready or if it happens in the future
		// (greater than or equal to the current sample count).
		if w.time >= wr.nwrite || !has(w.reg, ready) {
			break
		}

		w.reg &^= ready

		wr.write(w.reg, w.data)

		wr.cur = (wr.cur + 1) % BufferSize
	}

	wr.nwrite++
}
