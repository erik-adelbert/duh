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
	"math/bits"
)

const (
	YMF262ClockHz    = 14318180 // 14.31818 MHz
	YMF262ClockDiv   = 288
	YMF262SampleRate = 49716 // 49.716 kHz
)

const (
	FracBits = 10 // 10-bit fractional part

	OPL3Mode = 1
)

const (
	channelCount = 18
	slotCount    = 36
)

type ChannelCallback = func(id uint8, side int, sample int32)

type Chip struct {
	timer uint16

	rateRatio int32

	sampleCount int32

	samples [2][4]int16

	chans [channelCount]channel
	slots [slotCount]slot

	zero8  *uint8
	zero16 *int16

	envelopeTimer

	mode uint8 // newm new/legacy mode register

	rhy uint8 // rhythm mode register
	nts uint8 // note select register

	vibPos   uint8
	vibShift uint8

	trem      uint8
	tremPos   uint8
	tremShift uint8
	tremDirty bool

	rhythmMode

	noise   uint32
	noiseHH uint32
	noiseSD uint32

	mixes [2][]*channel // left, right

	mixBuffer [4]int32
	mixDirty  bool

	writeID uint32

	writeBuf wring

	channelCB ChannelCallback
}

func NewChip(sampleRate int) *Chip {
	return NewChipWithClock(sampleRate, YMF262ClockHz)
}

func NewChipWithClock(sampleRate, clockHz int) (chip *Chip) {
	if clockHz <= 0 {
		clockHz = YMF262ClockHz
	}

	if sampleRate <= 0 {
		// round to nearest integer
		sampleRate = (clockHz + YMF262ClockDiv/2) / YMF262ClockDiv
	}

	ck64 := int64(clockHz)
	cd64 := int64(YMF262ClockDiv)
	sr64 := int64(sampleRate)

	rateRatio := int32(
		(sr64 << FracBits) / (ck64 / cd64),
	)

	chip = new(Chip{
		rateRatio: rateRatio,

		zero16: new(int16),
		zero8:  new(uint8),

		tremShift: 4,
		vibShift:  1,

		noise: 1,

		mixes: [2][]*channel{
			make([]*channel, 0, channelCount),
			make([]*channel, 0, channelCount),
		},

		mixDirty: true,
	})

	chip.writeBuf.write = chip.WriteReg

	for i := range chip.slots {
		chip.slots[i] = slot{
			Chip: chip,

			mod: slink{chip.zero16, nil},

			egen: envGenRelease,

			erout: 0x1ff,
			eout:  0x1ff,

			trem: chip.zero8,

			id: uint8(i),
		}
	}

	for i := range chip.chans {
		ch := &chip.chans[i]
		ch.id = uint8(i)

		ch.kind = Chan2op

		ch.cha = 0xffff
		ch.chb = 0xffff

		ch.out = [4]slink{
			{chip.zero16, nil},
			{chip.zero16, nil},
			{chip.zero16, nil},
			{chip.zero16, nil},
		}

		slotID := chanMapping[i]

		ch.slots[0] = &chip.slots[slotID]
		ch.slots[1] = &chip.slots[slotID+3]

		chip.slots[slotID].channel = ch
		chip.slots[slotID+3].channel = ch

		switch {
		case i%9 < 3:
			ch.pair = &chip.chans[i+3]
		case i%9 < 6:
			ch.pair = &chip.chans[i-3]
		}

		ch.Chip = chip

		ch.patchAlgorithm()
	}

	return
}

func (c *Chip) SetChannelCallback(cb ChannelCallback) {
	c.channelCB = cb
}

func (c *Chip) Reset(sampleRate int) {
	*c = *NewChip(sampleRate)
}

func (c *Chip) ResetWithClock(sampleRate, clockHz int) {
	*c = *NewChipWithClock(sampleRate, clockHz)
}

func (c *Chip) Resample4Ch(out []int16) {
	const (
		old = iota
		nxt
	)

	for c.sampleCount >= c.rateRatio {
		c.samples[old] = c.samples[nxt]

		c.generate4Ch(c.samples[nxt][:])
		c.sampleCount -= c.rateRatio
	}

	for i := range 4 {
		out[i] = lerp(
			c.samples[old][i], c.samples[nxt][i], c.sampleCount, c.rateRatio,
		)
	}

	c.sampleCount += (1 << FracBits)
}

func (c *Chip) Resample2Ch(out []int16) {
	var samples [4]int16

	c.Resample4Ch(samples[:])

	out[0] = samples[0]
	out[1] = samples[1]
}

func (c *Chip) Stream4Ch(out0, out1 []int16, size int) {
	var samples [4]int16

	for i := 0; i < size; i += 2 {
		c.Resample4Ch(samples[:])

		out0[i], out0[i+1] = samples[0], samples[1]
		out1[i], out1[i+1] = samples[2], samples[3]
	}
}

func (c *Chip) Stream2Ch(out []int16, size int) {
	var samples [2]int16

	for i := 0; i < size; i += 2 {
		c.Resample2Ch(samples[:])

		out[i], out[i+1] = samples[0], samples[1]
	}
}

func (c *Chip) WriteReg(reg16 uint16, data uint8) {
	c.writeID++

	if c.writeID == 0 {
		// handle overflow
		for i := range c.slots {
			c.slots[i].dormantID = 0
		}

		c.writeID = 1
	}

	bank := int32(bit(8, reg16))

	// decode the register address
	lo8 := int32(reg16 & 0xff)

	rhi := lo8 & 0xf0 // base register address

	rlo := lo8 & 0xf // channel/mode select

	ad := adSlot[lo8&0x1f] // A/D slot index

	// golbal register writes, treat unsupported as nop
	if rhi == 0 {
		switch {
		case bank == 1 && rlo == 4:
			c.set4op(data) // 2x2/4-op mode select
		case bank == 1 && rlo == 5:
			c.mode = data & 1 // OPL3Mode
		case bank == 0 && rlo == 8:
			c.nts = bit(6, data) // note select
		}

		return
	}

	// rhythm mode write
	if reg16 == 0xbd {
		tremShift := uint8(2)

		if bit(7, data) == 0 {
			tremShift = 4
		}

		if tremShift != c.tremShift {
			c.tremShift = tremShift
			c.tremDirty = true
		}

		vibShift := bit(6, data) ^ 1

		if vibShift != c.vibShift {
			c.vibShift = vibShift

			for i := range c.slots {
				c.slots[i].updatePhase()
			}
		}

		c.updateDrum(data)

		return
	}

	// channel writes
	if rlo < 9 {
		const onFlag = 0x20

		// valid channel, write!
		cid := 9*bank + rlo // adjust for bank

		switch rhi {
		case 0xa0:
			c.chans[cid].writeA0(data)
		case 0xb0:
			c.chans[cid].writeB0(data)

			if has(data, onFlag) {
				c.chans[cid].keyOn()
			} else {
				c.chans[cid].keyOff()
			}
		case 0xc0:
			c.chans[cid].writeC0(data)
		}
	}

	// slot writes
	if ad >= 0 {
		// valid ID, write!
		sid := 18*bank + ad // adjust for bank

		switch rhi {
		case 0x20, 0x30:
			c.slots[sid].write20(data)
		case 0x40, 0x50:
			c.slots[sid].write40(data)
		case 0x60, 0x70:
			c.slots[sid].write60(data)
		case 0x80, 0x90:
			c.slots[sid].write80(data)
		case 0xe0, 0xf0:
			c.slots[sid].writeE0(data)
		}
	}
}

func (c *Chip) QueueReg(reg uint16, data uint8) {
	c.writeBuf.push(reg, data)
}

func (c *Chip) generate4Ch(out []int16) {

	out[1] = clip16(c.mixBuffer[1])
	out[3] = clip16(c.mixBuffer[3])

	if c.mixDirty {
		c.rebuildMixLists()
	}

	// advanve the noise LFSR
	c.makeNoise()

	wid := c.writeID

	for i := range 7 {
		ch := &c.chans[i]
		ch.processSlots(wid, false)
	}

	// 7, 8 may be the drum channels
	c.chans[7].processSlots(wid, true)
	c.chans[8].processSlots(wid, true)

	for i := 9; i < 18; i++ {
		ch := &c.chans[i]
		ch.processSlots(wid, false)
	}

	c.mix(mixLeft)

	out[0] = clip16(c.mixBuffer[0])
	out[2] = clip16(c.mixBuffer[2])

	c.advanceEffects()

	c.advanceEnvelope()

	c.mix(mixRight)

	c.writePending()
}

func (c *Chip) writePending() {
	c.writeBuf.flush()
}

func (c *Chip) advanceEffects() {
	// advance tremolo
	todo := c.tremDirty

	if c.timer&0x3f == 0x3f {
		c.trem = (c.trem + 1) % 210

		todo = true
	}

	if todo {
		if c.trem < 105 {
			c.trem = c.tremPos >> c.tremShift
		} else {
			c.trem = (210 - c.tremPos) >> c.tremShift
		}

		c.tremDirty = false
	}

	// advance vibrato
	todo = c.timer&0x3ff == 0x3ff

	if todo {
		c.vibPos = (c.vibPos + 1) & 7
	}

	c.timer++
}

func (c *Chip) advanceEnvelope() {
	et := &c.envelopeTimer

	switch {
	case et.state != 0:
		et.add = 0

		timerLo := et.timer & 0x1fff

		if timerLo > 0 {
			shift := bits.TrailingZeros64(timerLo)
			et.add = uint8(shift + 1)
		}

		et.timerLo = uint8(et.timer & 3)

		fallthrough // advance the timer if the state is true
	case et.timerRem > 0:
		et.timerRem = 0

		// handle overflow ???
		if et.timer == 0xfffffffff {
			et.timerRem = 1
		}

		et.timer++
	}

	et.state ^= 1
}

func (c *Chip) set4op(bitmask uint8) {
	for i := range 6 {
		cid := i

		if i >= 3 {
			cid += 9 - 3
		}

		if has(bitmask, 1<<i) {
			c.chans[cid+0].kind = Chan4op
			c.chans[cid+3].kind = Chan4op2

			c.chans[cid+0].updateAlgorithm()
		} else {
			c.chans[cid+0].kind = Chan2op
			c.chans[cid+3].kind = Chan2op

			c.chans[cid+0].updateAlgorithm()
			c.chans[cid+3].updateAlgorithm()
		}
	}
}

func (c *Chip) updateDrum(data uint8) {
	link0 := func() slink {
		return slink{c.zero16, nil}
	}

	linkOut := func(ch *channel, slot int) slink {
		return slink{&ch.slots[slot].out, ch.slots[slot]}
	}

	const rhythmFlag = 0x20

	c.rhy = data & 0x3f

	if !has(data, rhythmFlag) {
		for i := 6; i < 9; i++ {
			c.chans[i].kind = Chan2op

			c.chans[i].patchAlgorithm()

			c.chans[i].slots[0].keyOff(KeyDrum)
			c.chans[i].slots[1].keyOff(KeyDrum)
		}

		return
	}

	ch6 := &c.chans[6]

	ch6.out[0] = linkOut(ch6, 0)
	ch6.out[1] = linkOut(ch6, 1)
	ch6.out[2] = link0()
	ch6.out[3] = link0()

	ch7 := &c.chans[7]
	ch7.out[0] = linkOut(ch7, 0)
	ch7.out[1] = linkOut(ch7, 0)
	ch7.out[2] = linkOut(ch7, 1)
	ch7.out[3] = linkOut(ch7, 1)

	ch8 := &c.chans[8]
	ch8.out[0] = linkOut(ch8, 0)
	ch8.out[1] = linkOut(ch8, 0)
	ch8.out[2] = linkOut(ch8, 1)
	ch8.out[3] = linkOut(ch8, 1)

	for i := 6; i < 9; i++ {
		c.chans[i].kind = ChanDrum
	}

	ch6.patchAlgorithm()
	ch7.patchAlgorithm()
	ch8.patchAlgorithm()

	type keyParms struct {
		ch           *channel
		slot0, slot1 uint8
	}

	keys := []keyParms{
		{ch7, 0, 0},
		{ch8, 1, 1},
		{ch8, 0, 0},
		{ch7, 1, 1},
		{ch6, 0, 1},
	}

	for i := range keys {
		flag := uint8(1 << (i + 1))

		ch := keys[i].ch
		slot0, slot1 := keys[i].slot0, keys[i].slot1

		if c.rhy&flag != 0 {
			ch.slots[slot0].keyOn(KeyDrum)
			ch.slots[slot1].keyOn(KeyDrum)
		} else {
			ch.slots[slot0].keyOff(KeyDrum)
			ch.slots[slot1].keyOff(KeyDrum)
		}
	}

}

func (c *Chip) rebuildMixLists() {
	c.mixes[mixLeft] = c.mixes[mixLeft][:0]
	c.mixes[mixRight] = c.mixes[mixRight][:0]

	for i := range c.chans {
		ch := &c.chans[i]

		if ch.nout == 0 {
			continue
		}

		if ch.cha|ch.chc != 0 {
			c.mixes[mixLeft] = append(c.mixes[mixLeft], ch)
		}

		if ch.chb|ch.chd != 0 {
			c.mixes[mixRight] = append(c.mixes[mixRight], ch)
		}
	}

	c.mixDirty = false
}

// makeNoise advances the noise LFSR for the whole sample up front (36 steps, one per slot), capturing
// the bits the hh (slot 13) and sd (slot 16) operators read. This way slot processing does not touch
// the LFSR so it doesn't need to run in strict slot order.
func (c *Chip) makeNoise() {
	s := c.noise

	// Bit-tapped LFSR, the 36 feedback bits are computable word-parallel as:
	// fb_i = s_i ^ s_{i+14} for i in [0,8],
	// fb_i = s_i ^ fb_{i-9} for i in [9,22],
	// fb_i = fb_{i-23} ^ fb_{i-9} for i >= 23,
	//
	//	and the state after 36 steps is fb_13..fb_35. The hh/sd taps only read bit 0 of the
	// intermediate state, which is s_13 / s_16.
	f0_8 := (s ^ (s >> 14)) & 0x1ff
	f9_17 := ((s >> 9) ^ f0_8) & 0x1ff
	f18_22 := ((s >> 18) ^ f9_17) & 0x1f
	f23_31 := f0_8 ^ ((f9_17 >> 5) | (f18_22 << 4))
	f32_35 := (f9_17 ^ f23_31) & 0x0f

	c.noiseHH = (s >> 13) & 1
	c.noiseSD = (s >> 16) & 1

	c.noise = ((f9_17 >> 4) & 0x1f) | (f18_22 << 5) | (f23_31 << 10) | (f32_35 << 19)
}

type mixSide int

const (
	mixLeft mixSide = iota
	mixRight
)

func (c *Chip) mix(side mixSide) {
	var mix [2]int32

	for _, ch := range c.mixes[side] {
		// select output channel
		chout := ch.outR
		if side == mixLeft {
			chout = ch.outL
		}

		// accumulate the outputs of the channel's slots
		acc := *chout[0]
		for i := 1; i < int(ch.nout); i++ {
			acc += *chout[i]
		}

		sample := func(x int16, mask uint16) int32 {
			return int32(int16(uint16(x) & mask))
		}

		// update the channel's meter with the accumulated output

		// mix channel output into the left/right mix buffers
		if side == mixLeft {
			mix[0] += sample(acc, ch.cha)
			mix[1] += sample(acc, ch.chc)

			if c.channelCB != nil {
				c.channelCB(ch.id, int(side), sample(acc, ch.cha|ch.chc))
			}
		} else {
			mix[0] += sample(acc, ch.chb)
			mix[1] += sample(acc, ch.chd)

			if c.channelCB != nil {
				c.channelCB(ch.id, int(side), sample(acc, ch.chb|ch.chd))
			}
		}
	}

	// store the mixed output into the chip's mix buffer
	// left : 0, 2 right : 1, 3
	c.mixBuffer[side+0] = mix[0]
	c.mixBuffer[side+2] = mix[1]
}

type envelopeTimer struct {
	state uint8 // 0=low, 1=high

	add uint8

	timer    uint64
	timerLo  uint8
	timerRem uint8
}

type rhythmMode struct {
	hithat
	topCymbal
}

type hithat struct {
	bit2, bit3, bit7, bit8 uint8
}

type topCymbal struct {
	bit3, bit5 uint8
}

var adSlot = [32]int32{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, -1, -1,
	0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, -1, -1,
	0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1,
}

var chanMapping = [18]int32{
	0, 1, 2, 6, 7, 8,
	12, 13, 14, 18, 19, 20,
	24, 25, 26, 30, 31, 32,
}
