// duh midi driver for OPL3
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

package mid

import (
	"github.com/erik-adelbert/duh/pkg/mid/model"
	"github.com/erik-adelbert/duh/pkg/opl3"
	"golang.org/x/exp/constraints"
)

var Operators = [...]uint16{
	// Channels 0-2
	0x000, 0x003, 0x001, 0x004, 0x002, 0x005, // operators  0, 3,  1, 4,  2, 5
	// Channels 3-5
	0x008, 0x00B, 0x009, 0x00C, 0x00A, 0x00D, // operators  6, 9,  7,10,  8,11
	// Channels 6-8
	0x010, 0x013, 0x011, 0x014, 0x012, 0x015, // operators 12,15, 13,16, 14,17
	// Same for second card
	0x100, 0x103, 0x101, 0x104, 0x102, 0x105, // operators 18,21, 19,22, 20,23
	0x108, 0x10B, 0x109, 0x10C, 0x10A, 0x10D, // operators 24,27, 25,28, 26,29
	0x110, 0x113, 0x111, 0x114, 0x112, 0x115, // operators 30,33, 31,34, 32,35
	// Channel 18
	0x010, 0x013, // operators 12,15
	// Channel 19
	0xFFF, 0x014, // operator 16
	// Channel 19
	0x012, 0xFFF, // operator 14
	// Channel 19
	0xFFF, 0x015, // operator 17
	// Channel 19
	0x011, 0xFFF, // operator 13

	// CMF Rhythm-mode percussions, snare and cymbal operators are inverted
	0x010, 0x013, // operators 12,15
	// Channel 19
	0x014, 0xFFF, // operator 16
	// Channel 19
	0x012, 0xFFF, // operator 14
	// Channel 19
	0x015, 0xFFF, // operator 17
	// Channel 19
	0x011, 0xFFF, // operator 13
}

var Channels = [...]int{
	0x000, 0x001, 0x002, 0x003, 0x004, 0x005, 0x006, 0x007, 0x008, // 0..8
	0x100, 0x101, 0x102, 0x103, 0x104, 0x105, 0x106, 0x107, 0x108, // 9..17 (secondary set)
	0x006, 0x007, 0x008, 0x008, 0x008, // hardware percussion
}

/*
   In OPL3 mode:
        0    1    2    6    7    8     9   10   11    16   17   18
      op0  op1  op2 op12 op13 op14  op18 op19 op20  op30 op31 op32
      op3  op4  op5 op15 op16 op17  op21 op22 op23  op33 op34 op35
        3    4    5                   13   14   15
      op6  op7  op8                 op24 op25 op26
      op9 op10 op11                 op27 op28 op29
   Ports:
       +0   +1   +2  +10  +11  +12  +100 +101 +102  +110 +111 +112
       +3   +4   +5  +13  +14  +15  +103 +104 +105  +113 +114 +115
       +8   +9   +A                 +108 +109 +10A
       +B   +C   +D                 +10B +10C +10D

   Percussion:
     bassdrum = op(0): 0xBD bit 0x10, operators 12 (0x10) and 15 (0x13) / channels 6, 6b
     snare    = op(3): 0xBD bit 0x08, operators 16 (0x14)               / channels 7b
     tomtom   = op(4): 0xBD bit 0x04, operators 14 (0x12)               / channels 8
     cym      = op(5): 0xBD bit 0x02, operators 17 (0x17)               / channels 8b
     hihat    = op(2): 0xBD bit 0x01, operators 13 (0x11)               / channels 7
*/

// Replaces adl[] when a CMF file is loaded
var DynamicInstruments []adlData

// Replaces adlins[] when a CMF file is loaded
var DynamicMetaInstruments []adlMeta

// GetAdlIns returns the ADLData for the given instrument number.
// If the high bit (DynamicInstrumentTag) is set, the number indexes
// into DynamicInstruments; otherwise it indexes into the static ADL table.
func GetAdlIns(id uint16) *adlData {
	if has(id, DynamicInstrumentTag) {
		i := int(id &^ DynamicInstrumentTag)

		if i >= len(DynamicInstruments) {
			return nil
		}

		return &DynamicInstruments[i]
	}

	if int(id) >= len(adlIns) {
		return nil
	}

	return &adlIns[id]
}

var (
	// BankSelect selects which bank table to use for MIDI→meta-instrument lookup.
	// ^uint(0) (i.e. ~0u) means "use the dynamic/CMF meta-instrument table".
	BankSelect uint = 0

	// Number of 4-op channels to enable, total across all cards.
	FourOpsCount int = 7

	// Number of OPL3 chips (cards) to emulate.
	CardCount int = 2

	Mode int = 0
)

const (
	ModeHighTremolo = 1 << iota
	ModeHighVibrato
	ModeADLPercussion
	ModeLogarithmicVolumes
	ModeCartoonersVolumes
	ModeScaleModulators
	ModeSoftPanning
)

func has[T constraints.Integer](flags, mode T) bool {
	return (flags & mode) != 0
}

const (
	PCMRate = 49716
	OPLBase = 0x388
)

// QuadOP types for OPL3 channels
const (
	Regular = iota
	QuadMaster
	QuadSlave
	PercBD
	PercSnare
	PercTom
	PercCrash
	PercHihat
	PercSlave
)

const (
	Flag2op       = 0x00
	Flag4op       = 0x01
	FlagPseudo4op = 0x02
	FlagSilence   = 0x04
	FlagFixedTone = 0x40

	FlagRMBassDrum = 0x08
	FlagRMSnare    = 0x10
	FlagRMTomTom   = 0x18
	FlagRMCymbal   = 0x20
	FlagRMHiHat    = 0x28
	FlagRhythmMask = 0x38
)

type OPL3 struct {
	CardCount, QuadOpsCount int
	Flags                   int

	Cards []*opl3.Device

	quads []int8 // QuadOP type for each channel

	ins []uint16 // index to adl[], cached, needed by Touch()
	pit []uint8  // value poked to B0, cached, needed by NoteOff()

	bds []int
	cmf bool

	vm     model.Volume
	master int    // MIDI master volume, set by SysEx
	mod    []bool // operators' multipliers rewritten by a frequency model
}

// setVolumeModel selects the volume and frequency model used by Touch and NoteOn.
func (o *OPL3) setVolumeModel(vm model.Volume) {
	if vm == nil {
		vm = &model.Generic{}
	}

	o.vm = vm
}

// SetMasterVolume sets the master volume fed to the volume model.
func (o *OPL3) SetMasterVolume(v int) {
	o.master = v
}

func newOPL3(cardCount, quadOpsCount, flags int) *OPL3 {
	const (
		ncpc  = 23 // number of channels per OPL3 card
		perc0 = 18
	)

	nchan := cardCount * ncpc

	o := &OPL3{
		CardCount:    cardCount,
		QuadOpsCount: quadOpsCount,
		Flags:        flags,

		Cards: make([]*opl3.Device, cardCount),
		bds:   make([]int, cardCount),

		ins: make([]uint16, nchan),
		pit: make([]uint8, nchan),
		mod: make([]bool, nchan),

		quads: make([]int8, nchan),

		master: masterVolumeDefault,
	}

	o.setVolumeModel(&model.Generic{})

	if has(flags, ModeLogarithmicVolumes) {
		o.setVolumeModel(&model.Native{})
	}

	for i := range o.Cards {
		for j := range 5 {
			o.quads[i*ncpc+perc0+j] = PercSlave
		}
	}

	init := [...]int{
		0x004, 96, 0x004, 128, // Pulse timer
		0x105, 0, 0x105, 1, 0x105, 0, // Pulse OPL3 enable
		0x001, 32, 0x105, 1, // Enable wave, OPL3 extensions
		0x08, 0, // CSW/Note Sel
	}

	nquad := quadOpsCount
	for i := range cardCount {
		o.Cards[i] = opl3.NewDevice(PCMRate)

		for j := range Channels[:18] {
			o.poke(i, 0xB0+Channels[j], 0x00)
		}

		for j := range len(init) / 2 {
			o.poke(i, init[2*j], init[2*j+1])
		}

		if has(flags, ModeHighTremolo) {
			o.bds[i] = 0x80
		}

		if has(flags, ModeHighVibrato) {
			o.bds[i] += 0x40
		}

		if has(flags, ModeADLPercussion) {
			o.bds[i] += 0x20
		}

		o.poke(i, 0xBD, o.bds[i])

		nf := min(nquad, 6)
		nquad -= nf

		o.poke(i, 0x104, (1<<nf)-1)
	}

	if has(flags, ModeADLPercussion) {
		for i := range cardCount {
			for j := range 5 {
				o.quads[i*ncpc+perc0+j] = int8(j + 3)
			}

			for j := range 3 {
				o.quads[i*ncpc+6+j] = PercSlave
			}
		}
	}

	ii := 0
	for i := range quadOpsCount {
		o.quads[ii] = QuadMaster
		o.quads[ii+3] = QuadSlave

		switch i % 6 {
		case 0, 1, 3, 4:
			ii++
		case 2:
			ii += 7
		case 5:
			ii += 12
		}
	}

	o.silence()

	// fmt.Fprintf(os.Stderr, "opl3 CardCount=%d Cards=%d ins=%d QuadOps=%d\n",
	// 	o.CardCount, len(o.Cards), len(o.ins), len(o.QuadOps))

	return o
}

func (o *OPL3) chanCount() int {
	return len(o.ins)
}

func (o *OPL3) operator(ii, slot int) uint16 {
	index := 2*ii + slot
	if o.cmf && ii >= 18 {
		index += 10
	}
	return Operators[index]
}

func (o *OPL3) setBankMode(setup uint16, rhythm bool) {
	for card := range o.Cards {
		o.bds[card] &^= 0xe0
		if setup&(1<<8) != 0 || has(o.Flags, ModeHighTremolo) {
			o.bds[card] |= 0x80
		}
		if setup&(1<<9) != 0 || has(o.Flags, ModeHighVibrato) {
			o.bds[card] |= 0x40
		}
		if rhythm {
			o.bds[card] |= 0x20
		}
		o.poke(card, 0xBD, o.bds[card])
		for i := range 3 {
			o.quads[card*23+6+i] = Regular
			if rhythm {
				o.quads[card*23+6+i] = PercSlave
			}
		}
		for i := range 5 {
			o.quads[card*23+18+i] = PercSlave
			if rhythm {
				o.quads[card*23+18+i] = int8(PercBD + i)
			}
		}
	}
}

func (o *OPL3) poke(card, addr, value int) {
	data := uint8(value)
	bank, reg := uint8((addr>>8)&0xF), uint8(addr&0xFF)

	o.Cards[card].WriteRegister(bank, reg, data, true)
}

func (o *OPL3) noteOff(c int) {
	card, ii := c/23, c%23

	if ii >= 18 {
		o.bds[card] &^= 0x10 >> (ii - 18)

		o.poke(card, 0xBD, o.bds[card])

		return
	}

	o.poke(card, 0xB0+Channels[ii], int(o.pit[c]&0xDF))
}

// noteOn keys a channel on at the given tone (MIDI note plus bend, in semitones).
func (o *OPL3) noteOn(c int, tone float64) {
	card, ii := c/23, c%23

	tone = max(tone, 0)

	var off uint32

	x := int(o.vm.Freq(tone, &off))

	ch := Channels[ii]
	if o.cmf && ii >= 21 {
		ch = 0xFFF
	}

	if ii < 18 {
		x |= 0x2000

		o.mulOffset(c, int(off))
	}

	if ch != 0xFFF {
		o.poke(card, 0xA0+ch, x&0xFF)

		pit := x >> 8

		o.pit[c] = uint8(pit)
		o.poke(card, 0xB0+ch, pit)
	}
	if ii >= 18 {
		o.bds[card] |= 0x10 >> (ii - 18)
		o.poke(card, 0xBD, o.bds[card])
	}
}

// mulOffset applies the multiplier offset requested by the frequency model,
// or restores the patch's own multipliers once it is no longer needed.
func (o *OPL3) mulOffset(c, offset int) {
	card, ii := c/23, c%23

	if offset == 0 && !o.mod[c] {
		return
	}

	ains := GetAdlIns(o.ins[c])
	ops := [2]int{int(ains.ModulatorE862) & 0xFF, int(ains.CarrierE862) & 0xFF}

	for k, v := range ops {
		addr := int(o.operator(ii, k))
		if addr == 0xFFF {
			continue
		}

		if offset > 0 {
			mul := v & 0x0F
			if mul+offset > 0x0F {
				offset, mul = 0, 0x0F
			}

			v = v&0xF0 | (mul + offset)
		}

		o.poke(card, 0x20+addr, v)
	}

	o.mod[c] = offset > 0
}

func (o *OPL3) touch(c, vel, vol, exp, bright int, isDrum bool) {
	var i0, i1 uint16

	i := o.ins[c]
	ains := GetAdlIns(i)

	mode := model.OPLVoice_MODE_2op_AM
	switch o.quads[c] {
	case Regular, PercBD:
		mode = int(ains.FeedConn & 1) // 2-op FM or 2-op AM
	case QuadMaster:
		i0 = i
		i1 = o.ins[c+3]

		mode = 2 // 4-op xx-xx ops 1&2
		mode += int(GetAdlIns(i0).FeedConn&1 + 2*(GetAdlIns(i1).FeedConn&1))
	case QuadSlave:
		i0 = o.ins[c-3]
		i1 = i

		mode = 6 // 4-op xx-xx ops 3&4
		mode += int(GetAdlIns(i0).FeedConn&1 + 2*(GetAdlIns(i1).FeedConn&1))
	}

	todos := [10][2]bool{
		{false, true},  // 2 op FM
		{true, true},   // 2 op AM
		{false, false}, // 4 op FM-FM ops 1&2
		{true, false},  // 4 op AM-FM ops 1&2
		{false, true},  // 4 op FM-AM ops 1&2
		{true, false},  // 4 op AM-AM ops 1&2
		{false, true},  // 4 op FM-FM ops 3&4
		{false, true},  // 4 op AM-FM ops 3&4
		{false, true},  // 4 op FM-AM ops 3&4
		{true, true},   // 4 op AM-AM ops 3&4
	}

	card, ii := c/23, c%23

	op0 := o.operator(ii, 0)
	op1 := o.operator(ii, 1)

	x := int(ains.Modulator40)
	y := int(ains.Carrier40)

	var m model.OPLModel

	m.Set(model.OPLVelocity, vel)
	m.Set(model.OPLChanVolume, vol)
	m.Set(model.OPLChanExpression, exp)
	m.Set(model.OPLMasterVolume, o.master)
	m.Set(model.OPLVoiceMode, mode)
	m.Set(model.OPLFbconn, int(ains.FeedConn))
	m.Set(model.OPLModN, x&63)
	m.Set(model.OPLCarN, y&63)
	m.EnableMod(todos[mode][0] || has(o.Flags, ModeScaleModulators))
	m.EnableCar(todos[mode][1] || has(o.Flags, ModeScaleModulators))
	m.EnableDrum(isDrum)

	o.vm.Volume(&m)

	if isDrum {
		const drumLoudnessBias = 8 // TL steps; subtracting makes the operator louder

		cur := m.Get(model.OPLCarN)
		m.Set(model.OPLCarN, max(0, cur-drumLoudnessBias))

		// In additive mode, the modulator is audible too.
		if ains.FeedConn&1 != 0 {
			cur = m.Get(model.OPLModN)
			m.Set(model.OPLModN, max(0, cur-drumLoudnessBias))
		}
	}

	if bright != 127 && !isDrum {
		bright = model.Brightness(bright)

		// Operators not scaled by the volume model follow the brightness.
		if !m.IsMod() {
			m.Set(model.OPLModN, 63-bright+bright*m.Get(model.OPLModN)/63)
		}

		if !m.IsCar() {
			m.Set(model.OPLCarN, 63-bright+bright*m.Get(model.OPLCarN)/63)
		}
	}

	if op0 != 0xFFF {
		o.poke(card, 0x40+int(op0), x&0xC0|m.Get(model.OPLModN)&63)
	}

	if op1 != 0xFFF {
		o.poke(card, 0x40+int(op1), y&0xC0|m.Get(model.OPLCarN)&63)
	}
}

// // Touch updates a channel's levels from its velocity, channel volume,
// // expression and brightness.
// func (o *OPL3) Touch(c, vel, vol, exp, bright int, isDrum bool) {
// 	o.touch(c, vel, vol, exp, bright, isDrum)
// }

// mute sets the maximum attenuation on a channel's operators.
func (o *OPL3) mute(c int) {
	card, ii := c/23, c%23
	ains := GetAdlIns(o.ins[c])

	if op0 := o.operator(ii, 0); op0 != 0xFFF {
		o.poke(card, 0x40+int(op0), int(ains.Modulator40)|63)
	}

	if op1 := o.operator(ii, 1); op1 != 0xFFF {
		o.poke(card, 0x40+int(op1), int(ains.Carrier40)|63)
	}
}

func (o *OPL3) patch(c int, i uint16) {
	card, ii := c/23, c%23

	data := [4]int{0x20, 0x60, 0x80, 0xE0}

	o.ins[c] = i

	op0 := int(o.operator(ii, 0))
	op1 := int(o.operator(ii, 1))

	ains := GetAdlIns(i)

	mod := int(ains.ModulatorE862)
	car := int(ains.CarrierE862)

	for i := range data {
		if op0 != 0xFFF {
			o.poke(card, data[i]+op0, mod&0xFF)
		}

		if op1 != 0xFFF {
			o.poke(card, data[i]+op1, car&0xFF)
		}

		mod >>= 8
		car >>= 8
	}
}

// pan places a channel in the stereo field from a MIDI position (0..127).
// With soft panning, the channel feeds both sides and the emulator weighs
// them; otherwise the position is squashed to left, right or both.
func (o *OPL3) pan(phys, value int) {
	card, ii := phys/23, phys%23

	if ii >= 21 || Channels[ii] == 0xFFF {
		return
	}

	reg := Channels[ii]
	pan := 0x30

	if has(o.Flags, ModeSoftPanning) {
		o.Cards[card].WritePan(uint8(reg>>8), uint8(reg), uint8(value))
	} else {
		pan = 0

		if value < 64+16 {
			pan |= 0x10
		}

		if value >= 64-16 {
			pan |= 0x20
		}
	}

	o.poke(card, 0xC0+reg, pan|int(GetAdlIns(o.ins[phys]).FeedConn))
}

// silence all OPL channels.
func (o *OPL3) silence() {
	for i := range o.ins {
		o.noteOff(i)
		o.mute(i)
	}
}
