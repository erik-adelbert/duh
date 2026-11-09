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

package main

import (
	"math"

	mid "github.com/erik-adelbert/duh/pkg/mid"
	"github.com/erik-adelbert/duh/pkg/opl3"
)

const (
	ChanCount   = 23
	RMChanCount = 5
)

type Synth struct {
	*opl3.Chip

	note, curnote uint
	playnote      int

	detune float64

	offs [2]int16
	x    [2]uint

	quad, pseudo4, rhythm bool
}

func NewSynth() *Synth {
	return &Synth{
		Chip: opl3.NewChip(opl3.YMF262SampleRate),
	}
}

func (s *Synth) Reset() {
	s.Chip = opl3.NewChip(opl3.YMF262SampleRate)

	init := [...]uint16{
		0x004, 96, 0x004, 128, // Pulse timer
		0x105, 0, 0x105, 1, 0x105, 0, // Pulse OPL3 enable
		0x001, 32, 0x105, 1, // Enable wave, OPL3 extensions
	}

	for i := range chmap {
		s.WriteReg(0xB0+chmap[i], 0x00)
	}

	for i := 0; i < len(init); i += 2 {
		s.WriteReg(init[i], uint8(init[i+1]))
	}
}
func (s *Synth) SetInstrument(banks *mid.BankDB, ie *mid.InsEntry) {
	// fmt.Println("setting instrument", ie.ID)
	s.pseudo4 = mid.IsPseudo4Op(ie.Flags)
	s.quad = mid.Is4Op(ie.Flags) && !s.pseudo4
	s.rhythm = has(ie.Flags, mid.FlagRhythmMask)

	clear(s.x[:])

	nop := 2
	if s.quad || s.pseudo4 {
		nop = 4
	}

	ops := [4]mid.OP{
		*banks.Ops[ie.Ops[0]],
		*banks.Ops[ie.Ops[1]],
	}

	if nop == 4 {
		ops[2] = *banks.Ops[ie.Ops[2]]
		ops[3] = *banks.Ops[ie.Ops[3]]
	}

	s.playnote = int(ie.DrumKey)
	if s.playnote >= 128 {
		s.playnote -= 128
	}
	s.playnote = max(s.playnote, 25)

	s.note = uint(nop) / 2

	if s.curnote = s.note; s.quad {
		s.curnote = 1
	}

	s.offs = [2]int16{
		int16(ie.Note.Off0),
		int16(ie.Note.Off1),
	}

	if s.pseudo4 {
		s.detune = ie.SecVoice
	}

	var data uint8

	if s.quad {
		data = 0x3F
	}

	s.WriteReg(0x104, data)

	for i := range ops {
		ops[i].E862 &= 0xFFFFFF3F
	}

	regs16 := [...]uint16{
		0x20, 0x60, 0x80, 0xE0,
	}

	for i := range s.note {
		ii := 2 * i

		jj := ii
		if s.quad {
			jj = 6 * i
		}

		o0, o1 := opmap[jj], opmap[jj+1]

		mod0, mod1 := ops[ii].E862, ops[ii+1].E862
		car0, car1 := ops[ii].D40, ops[ii+1].D40
		fbconn := (ie.FBConn >> (8 * i)) & 0xFF

		for j := range 4 {
			s.WriteReg(regs16[j]+o0, uint8(mod0&0xFF))
			s.WriteReg(regs16[j]+o1, uint8(mod1&0xFF))

			mod0 >>= 8
			mod1 >>= 8
		}

		ch := i
		if s.quad {
			ch = 3 * i
		}

		s.WriteReg(0xC0+chmap[ch], uint8(fbconn|0x30))

		s.WriteReg(0x40+o0, uint8(car0&0xFF))
		s.WriteReg(0x40+o1, uint8(car1&0xFF))
	}
}

func (s *Synth) KeyOn() {
	clear(s.x[:])

	for i := range s.curnote {
		hz := 172.00093 * math.Exp(
			0.057762265*float64(s.playnote+int(s.offs[i])),
		)

		hz = min(hz, 131071)

		s.x[i] = 0x2000

		for hz >= 1023.5 {
			hz /= 2
			s.x[i] += 0x400
		}

		s.x[i] += uint(hz + 0.5)

		off := chmap[i]
		data := s.x[i]

		s.WriteReg(0xA0+off, lo8(data))
		s.WriteReg(0xB0+off, hi8((data)))
	}
}

func (s *Synth) KeyOff() {
	for i := range s.curnote {
		off := chmap[i]
		data := s.x[i]

		s.WriteReg(0xB0+off, hi8(data)&0xDF)
	}
}

var opmap = [...]uint16{
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

	//==For Rhythm-mode percussions
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

	//==For Rhythm-mode percussions in CMF, snare and cymbal operators are inverted!
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

var chmap = [...]uint16{
	0x000, 0x001, 0x002, 0x003, 0x004, 0x005, 0x006, 0x007, 0x008, // 0..8
	0x100, 0x101, 0x102, 0x103, 0x104, 0x105, 0x106, 0x107, 0x108, // 9..17 (secondary set)
	0x006, 0x007, 0x008, 0x008, 0x008, // <- hw percussions, hihats and cymbals using tom-tom's channel as pitch source
}
