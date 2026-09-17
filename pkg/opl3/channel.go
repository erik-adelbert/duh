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

import (
	"math"
)

const (
	Chan2op = iota
	Chan4op
	Chan4op2
	ChanDrum
)

// channel represents an OPL3 channel, which can be a 2-operator,
// 4-operator, or drum channel.
type channel struct {
	id uint8

	*Chip

	slots [2]*slot
	pair  *channel

	out  [4]slink
	outL [4]*int16
	outR [4]*int16
	nout uint8

	kind     uint8
	fic10    uint16 // 10-bit frequency increment aka oct aka F_NUM
	blk      uint8
	fbk      uint8
	con      uint8
	alg      uint8
	ksv      uint8
	cha, chb uint16
	chc, chd uint16
}

func (c *channel) processSlots(gen uint32, withRhythm bool) {
	fbk := c.fbk

	c.slots[0].processInline(gen, fbk, withRhythm)
	c.slots[1].processInline(gen, fbk, withRhythm)
}

func (c *channel) updateDelayed() {
	chip := c.Chip

	for i := range 4 {
		out := c.out[i]

		outL, outR := out.i16(), out.i16()

		if out != nilSlink() && out.i16() != chip.zero16 {
			slot := out.from()

			if slot.id >= 15 {
				outL = &chip.slots[slot.id].prout
			}

			if slot.id >= 33 {
				outR = &chip.slots[slot.id].prout
			}
		}

		c.outL[i] = outL
		c.outR[i] = outR
	}
}

func (c *channel) setupAlgorithm() {
	link0 := func() slink {
		return slink{c.zero16, nil}
	}

	linkFeedBack := func(s *slot) slink {
		return slink{&s.fbmod, nil}
	}

	linkOut := func(s *slot) slink {
		return slink{&s.out, s}
	}

	switch {
	case c.kind == ChanDrum:
		if c.id == 7 || c.id == 8 {
			c.slots[0].mod = link0()
			c.slots[1].mod = link0()

			return
		}

		c.slots[0].mod = linkFeedBack(c.slots[0])

		c.slots[1].mod = linkOut(c.slots[0])
		if c.alg&1 == 1 {
			c.slots[1].mod = link0()
		}

	case has(c.alg, 8):
		// do nothing
	case has(c.alg, 4):
		c.pair.out[0] = link0()
		c.pair.out[1] = link0()
		c.pair.out[2] = link0()
		c.pair.out[3] = link0()

		c.pair.nout = 0

		c.out[1] = link0()
		c.out[2] = link0()
		c.out[3] = link0()

		switch c.alg & 3 {
		case 0:

			c.pair.slots[0].mod = linkFeedBack(c.pair.slots[0])
			c.pair.slots[1].mod = linkOut(c.pair.slots[0])

			c.slots[0].mod = linkOut(c.pair.slots[1])
			c.slots[1].mod = linkOut(c.slots[0])

			c.out[0] = linkOut(c.slots[1])

			c.nout = 1
		case 1:
			c.pair.slots[0].mod = linkFeedBack(c.pair.slots[0])
			c.pair.slots[1].mod = linkOut(c.pair.slots[0])

			c.slots[0].mod = link0()
			c.slots[1].mod = linkOut(c.slots[0])

			c.out[0] = linkOut(c.pair.slots[1])
			c.out[1] = linkOut(c.slots[1])

			c.nout = 2
		case 2:
			c.pair.slots[0].mod = linkFeedBack(c.pair.slots[0])
			c.pair.slots[1].mod = link0()

			c.slots[0].mod = linkOut(c.pair.slots[1])
			c.slots[1].mod = linkOut(c.slots[0])

			c.out[0] = linkOut(c.pair.slots[0])
			c.out[1] = linkOut(c.slots[1])

			c.nout = 2
		case 3:
			c.pair.slots[0].mod = linkFeedBack(c.pair.slots[0])
			c.pair.slots[1].mod = link0()

			c.slots[0].mod = linkOut(c.pair.slots[1])
			c.slots[1].mod = link0()

			c.out[0] = linkOut(c.pair.slots[0])
			c.out[1] = linkOut(c.slots[0])
			c.out[2] = linkOut(c.slots[1])

			c.nout = 3
		}
	default:
		c.slots[0].mod = linkFeedBack(c.slots[0])

		c.out[1] = link0()
		c.out[2] = link0()
		c.out[3] = link0()

		switch c.alg & 1 {
		case 0:
			c.slots[1].mod = linkOut(c.slots[0])

			c.out[0] = linkOut(c.slots[1])

			c.nout = 1
		case 1:
			c.slots[1].mod = link0()

			c.out[0] = linkOut(c.slots[0])
			c.out[1] = linkOut(c.slots[1])

			c.nout = 2
		}
	}
}

func (c *channel) patchAlgorithm() {

	c.setupAlgorithm()

	c.updateDelayed()
	if c.pair != nil {
		c.pair.updateDelayed()
	}

	c.mixDirty = true
}

func (c *channel) updateAlgorithm() {
	c.alg = c.con

	if c.mode == OPL3Mode && c.kind == Chan4op {
		c.alg = 8

		c.pair.alg = 4 | (c.con << 1) | c.pair.con
		c.pair.patchAlgorithm()

		return
	}

	if c.mode == OPL3Mode && c.kind == Chan4op2 {
		c.alg = 4 | (c.pair.con << 1) | c.con
		c.pair.alg = 8
	}

	c.patchAlgorithm()
}

func (c *channel) writeA0(data uint8) {
	if c.mode == OPL3Mode && c.kind == Chan4op2 {
		return
	}

	c.fic10 = (c.fic10 & 0x300) | uint16(data)

	c.ksv = uint8(
		(uint16(c.blk) << 1) | ((c.fic10 >> (9 - c.nts)) & 1),
	)

	c.slots[0].updateKSL()
	c.slots[1].updateKSL()

	c.slots[0].updateEnvRate()
	c.slots[1].updateEnvRate()

	c.slots[0].updatePhase()
	c.slots[1].updatePhase()

	if c.mode == OPL3Mode && c.kind == Chan4op {
		c.pair.fic10 = c.fic10
		c.pair.ksv = c.ksv

		c.pair.slots[0].updateKSL()
		c.pair.slots[1].updateKSL()

		c.pair.slots[0].updateEnvRate()
		c.pair.slots[1].updateEnvRate()

		c.pair.slots[0].updatePhase()
		c.pair.slots[1].updatePhase()
	}
}

func (c *channel) writeB0(data uint8) {
	if c.mode == OPL3Mode && c.kind == Chan4op2 {
		return
	}

	c.fic10 = (c.fic10 & 0xff) | (uint16(data&3) << 8)
	c.blk = (data >> 2) & 7
	c.ksv = uint8(
		uint16(c.blk<<1) | ((c.fic10 >> (9 - c.nts)) & 1),
	)

	c.slots[0].updateKSL()
	c.slots[1].updateKSL()

	c.slots[0].updateEnvRate()
	c.slots[1].updateEnvRate()

	c.slots[0].updatePhase()
	c.slots[1].updatePhase()

	if c.mode == OPL3Mode && c.kind == Chan4op {
		c.pair.fic10 = c.fic10
		c.pair.blk = c.blk
		c.pair.ksv = c.ksv

		c.pair.slots[0].updateKSL()
		c.pair.slots[1].updateKSL()

		c.pair.slots[0].updateEnvRate()
		c.pair.slots[1].updateEnvRate()

		c.pair.slots[0].updatePhase()
		c.pair.slots[1].updatePhase()
	}
}

func (c *channel) writeC0(data uint8) {
	c.fbk = (data & 0xe) >> 1
	c.con = (data & 1)

	c.updateAlgorithm()

	c.cha, c.chb, c.chc, c.chd = 0, 0, 0, 0

	if c.mode != OPL3Mode {
		c.cha, c.chb = math.MaxUint16, math.MaxUint16

		return
	}

	if has(data, 1<<4) {
		c.cha = math.MaxUint16
	}

	if has(data, 1<<5) {
		c.chb = math.MaxUint16
	}

	if has(data, 1<<6) {
		c.chc = math.MaxUint16
	}

	if has(data, 1<<7) {
		c.chd = math.MaxUint16
	}
}

const (
	KeyStd = 1 + iota
	KeyDrum
)

func (c *channel) keyOn() {
	c.slots[0].keyOn(KeyStd)
	c.slots[1].keyOn(KeyStd)

	if c.mode == OPL3Mode && c.kind == Chan4op {
		c.pair.slots[0].keyOn(KeyStd)
		c.pair.slots[1].keyOn(KeyStd)
	}
}

func (c *channel) keyOff() {
	c.slots[0].keyOff(KeyStd)
	c.slots[1].keyOff(KeyStd)

	if c.mode == OPL3Mode && c.kind == Chan4op {
		c.pair.slots[0].keyOff(KeyStd)
		c.pair.slots[1].keyOff(KeyStd)
	}
}
