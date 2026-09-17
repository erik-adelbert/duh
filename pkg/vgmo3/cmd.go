// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import "fmt"

const (
	OpWrite0    = 0x5e // write bank 0 (2 bytes)
	OpWrite1    = 0x5f // write bank 1 (2 bytes)
	OpWaitN     = 0x61 // wait n samples (2 bytes)
	OpWait735   = 0x62 // wait 735 samples (1 byte)
	OpWait882   = 0x63 // wait 882 samples (1 byte)
	OpWaitShort = 0x70 // wait n samples (1 byte, n = 0..15)
	OpEnd       = 0x66 // end of sound data (1 byte)
)

// Command represents a single VGM command, which can be a
// write to an OPL3 chip or a wait command.
type Command struct {
	op      byte
	reg     byte
	data    byte
	addr    offset
	nsample int // in samples
}

// Op returns the opcode of the command.
func (c Command) Op() byte {
	return c.op
}

// Reg returns the register of the command.
func (c Command) Reg() byte {
	return c.reg
}

// Data returns the data of the command.
func (c Command) Data() byte {
	return c.data
}

func (c Command) Addr() offset {
	return c.addr
}

// SampleCount returns the number of samples to wait for wait commands.
func (c Command) SampleCount() int {
	return c.nsample
}

// String returns a string representation of the command.
func (c Command) String() string {
	var nsample int

	addr := fmt.Sprintf("0x%05x", c.addr)

	sprintf := func(format string, a ...any) string {

		return addr + " " + fmt.Sprintf(format, a...)
	}

	switch c.op {
	case OpWrite0, OpWrite1:
		bank := 0

		if c.op == OpWrite1 {
			bank = 1
		}

		return sprintf("write\tbank%d %02x <- %02x", bank, c.reg, c.data)
	case OpWait735:
		nsample = 735

		fallthrough
	case OpWait882:
		if nsample == 0 {
			nsample = 882
		}

		fallthrough
	case OpWaitN:
		if nsample == 0 {
			nsample = c.nsample
		}

		return sprintf("wait\t%d", nsample)
	}

	if hi4(c.op) == OpWaitShort {
		nsample = int(lo4(c.op))
		return sprintf("wait\t%d", nsample)
	}

	if c.op == OpEnd {
		if c.nsample == -1 {
			return sprintf("end of stream")
		}

		return sprintf("loop\t0x%05x", c.nsample+2)
	}

	return sprintf("bad op\t%02x", c.op)
}
