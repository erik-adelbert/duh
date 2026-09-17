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
	"slices"
)

type fp123 = uint16 // 12.3 fixed-point representation

type slot struct {
	id uint8

	*channel
	*Chip

	mod  slink
	trem *uint8

	phaseGen

	dormantID uint32

	out   int16
	prout int16
	fbmod int16

	envGen

	regs

	key uint8
}

func (s *slot) isRhythm(withRhythm bool) bool {
	return withRhythm && slices.Contains([]uint8{13, 16, 17}, s.id)
}

func (s *slot) doProcess(fbk uint8, withRhythm bool) {

	if s.key == 0 && s.erout == 0x1ff && !s.isRhythm(withRhythm) {
		if fbk == 0 && s.phinc == 0 && s.out == 0 && *s.mod.i16() == 0 &&
			s.etks == 0 && *s.trem == 0 && s.phase == 0 && s.rvib == 0 &&
			s.rwf == 0 {

			s.fbmod = 0
			s.prout = 0
			s.eout = 0x1ff
			s.phrst = false
			s.egen = envGenRelease
			s.phout = 0

			return
		}

		s.calcFeedBack(fbk)

		s.eout = s.erout + s.etks + fp123(*s.trem)
		s.phrst = false
		s.egen = envGenRelease

		phinc := s.phinc
		if s.rvib != 0 {
			phinc = s.phincs[s.vibPos]
		}

		phase := uint16(s.phase >> 9)
		s.phase += phinc
		s.phout = phase

		s.genSilent()

		return
	}

	if s.egen == envGenSustain && s.key != 0 && s.erates[envGenSustain] == 0 {
		s.calcFeedBack(fbk)
		s.eout = s.erout + fp123(s.etks) + fp123(*s.trem)
		s.phrst = false

		if s.erout&0x1f8 == 0x1f8 {
			s.erout = 0x1ff
		}

		if s.rvib != 0 || s.isRhythm(withRhythm) {
			s.genPhase(withRhythm)
		} else {
			s.phout = uint16(s.phase >> 9)
			s.phase += s.phinc
		}

		s.genWaveform()

		return
	}

	s.calcFeedBack(fbk)
	s.calcEnvelope()
	s.genPhase(withRhythm)
	s.genWaveform()
}

func (s *slot) processStandard(fbk uint8) {
	s.doProcess(fbk, false)
}

func (s *slot) processRhythm(fbk uint8) {
	s.doProcess(fbk, true)
}

func (s *slot) processInline(writeID uint32, feedback uint8, withRhythm bool) {

	if s.dormantID == writeID {
		return
	}

	// Check if the slot is dormant (inactive) to potentially skip processing.
	// This is the "fast" in OPL3-Nuked-Fast.
	dormant := s.key == 0 && s.erout == 0x1ff && s.egen == envGenRelease
	dormant = dormant && !s.isRhythm(withRhythm)
	dormant = dormant && feedback == 0 && s.phinc == 0 && s.out == 0 && s.prout == 0
	dormant = dormant && *s.mod.i16() == 0 && s.etks == 0 && *s.trem == 0
	dormant = dormant && s.phase == 0 && s.rvib == 0 && s.rwf == 0

	if dormant {
		if s.trem == s.zero8 {
			mod, fbmod := s.mod, &s.fbmod
			from := mod.from()

			if mod.i16() == s.zero16 || mod.i16() == fbmod {
				s.dormantID = writeID
			} else if from != nil && from.dormantID == writeID {
				s.dormantID = writeID
			}
		}

		return
	}

	if withRhythm {
		s.processRhythm(feedback)
	} else {
		s.processStandard(feedback)
	}
}

func (s *slot) write20(data uint8) {
	s.trem = s.zero8

	if has(data, (1 << 7)) {
		s.trem = &s.Chip.trem
	}

	s.rvib = bit(6, data)
	s.regs.kind = bit(5, data)
	s.rksr = bit(4, data)

	s.erates[2] = 0
	if s.regs.kind == 0 {
		s.erates[2] = s.rrr
	}

	s.rmul = lo4(data)

	s.updateEnvRate()
	s.updatePhase()
}

func (s *slot) write40(data uint8) {
	s.rksl = hi2(data)
	s.rtl = lo6(data)

	s.updateKSL()
}

func (s *slot) write60(data uint8) {
	s.rar = hi4(data)
	s.rdr = lo4(data)

	s.erates[0] = s.rar
	s.erates[1] = s.rdr

	s.updateEnvRate()
}

func (s *slot) write80(data uint8) {
	s.rsl = hi4(data)

	if s.rsl == 0xf {
		s.rsl = 0x1f
	}

	s.rrr = lo4(data)

	s.erates[2] = 0
	if s.regs.kind == 0 {
		s.erates[2] = s.rrr
	}

	s.erates[3] = s.rrr

	s.updateEnvRate()
}

func (s *slot) writeE0(data uint8) {
	s.rwf = lo3(data)

	if s.mode != OPL3Mode {
		s.rwf = lo2(data)
	}
}

const phaseMask = (1 << 10) - 1 // 10-bit phase for 360° cycle

func (s *slot) genWaveform() {
	phase := uint16(int32(*s.mod.i16()) + int32(s.phout))
	phase &= phaseMask
	env := s.eout

	wf := sinROM[s.rwf][phase]
	neg := uint16(int16(wf) >> 15)

	logAmp := uint32(wf&0x7fff) + (uint32(env) << 3)

	s.out = int16(linexp(logAmp)) ^ int16(neg)
}

func (s *slot) genSilent() {
	phase := uint16(int32(*s.mod.i16()) + int32(s.phout))
	phase &= phaseMask

	wf := sinROM[s.rwf][phase]
	s.out = int16(wf) >> 15
}

func (s *slot) updateEnvRate() {
	s.eks = s.ksv >> ((s.rksr ^ 1) << 1)
	for i := range 4 {
		rate := uint8(int16(s.eks) + int16(s.erates[i])<<2)

		hi := hi6(rate)
		if has(hi, 0x10) {
			hi = 0xf
		}

		s.erateHis[i] = hi
		s.erateLos[i] = lo2(rate)
	}
}

func (s *slot) calcFeedBack(fbk uint8) {
	s.fbmod = 0

	if fbk != 0 {
		s.fbmod = (s.prout + s.out) >> (9 - s.fbk)
	}

	s.prout = s.out
}

func (s *slot) calcEnvelope() {
	var (
		e struct {
			rate  uint8
			shift uint8
			rout  fp123
			inc   int16
			off   uint8
		}

		reset bool
	)

	s.eout = s.erout + s.etks + fp123(*s.trem)

	e.rate = s.erates[s.egen]

	if s.key != 0 && s.egen == envGenRelease {
		reset = true
		e.rate = s.erates[0]
	}

	s.phrst = reset

	rateHi := s.erateHis[s.egen]
	rateLo := s.erateLos[s.egen]

	if reset {
		rateHi = s.erateHis[0]
		rateLo = s.erateLos[0]
	}

	e.shift = rateHi + s.add

	var shift uint8
	if e.rate != 0 {
		switch {
		case rateHi < 12:
			if s.state == 0 {
				break
			}

			switch e.shift {
			case 12:
				shift = 1
			case 13:
				shift = (rateLo >> 1) & 1
			case 14:
				shift = rateLo & 1
			}

		default:
			shift = (rateHi & 3) + envGenSteps[rateLo][s.timerLo]

			if has(shift, 4) {
				shift = 3
			} else if shift == 0 {
				shift = s.state
			}
		}
	}

	e.rout = s.erout

	if reset && rateHi == 0xf {
		e.rout = 0
	}

	if s.erout&0x1f8 == 0x1f8 {
		e.off = 1
	}

	if s.egen != envGenAttack && !reset && e.off != 0 {
		e.rout = 0x1ff
	}

	switch s.egen {
	case envGenAttack:
		switch {
		case s.erout == 0:
			s.egen = envGenDecay

		case s.key != 0 && shift > 0 && rateHi != 0xf:
			e.inc = int16(^s.erout >> (4 - shift))
		}

	case envGenDecay:
		if s.erout>>4 == fp123(s.rsl) {
			s.egen = envGenSustain
		} else if e.off == 0 && !reset && shift > 0 {
			e.inc = 1 << (shift - 1)
		}

	case envGenSustain, envGenRelease:
		if e.off == 0 && !reset && shift > 0 {
			e.inc = 1 << (shift - 1)
		}
	}

	s.erout = fp123((int32(e.rout) + int32(e.inc)) & 0x1ff)

	if reset {
		s.egen = envGenAttack
	}

	if s.key == 0 {
		s.egen = envGenRelease
	}
}

func (s *slot) updatePhase() {
	baseFreq := (uint32(s.fic10) << s.blk) >> 1
	s.phinc = (baseFreq * phincROM[s.rmul]) >> 1

	for i := range 8 { // 8 phase increments for vibrato
		finc10 := s.fic10

		span := int8((finc10 >> 7) & 7) // bits 7-9 of finc10

		switch {
		case i%4 == 0: // every 4th slot, reset span
			span = 0
		case i&1 != 0: // odd slots, apply vibrato
			span >>= 1
		}

		span >>= s.vibShift

		if bit(2, i) != 0 {
			span = -span
		}

		finc10 = uint16(int32(finc10) + int32(span))

		s.phincs[i] = (((uint32(finc10) << s.blk) >> 1) * phincROM[s.rmul]) >> 1
	}
}

func (s *slot) genPhase(withRhythm bool) {
	chip := s.Chip

	phinc := s.phinc
	if s.rvib != 0 {
		phinc = s.phincs[chip.vibPos]
	}

	phase := uint16(s.phase >> 9)
	if s.phrst {
		s.phase = 0
	}

	s.phase += phinc

	s.phout = phase

	if !withRhythm {
		return
	}

	// Rhythm mode

	if s.id == 13 {
		chip.bit2 = bit(2, phase)
		chip.hithat.bit3 = bit(3, phase)
		chip.bit7 = bit(7, phase)
		chip.bit8 = bit(8, phase)
	}

	if !has(chip.rhy, 0x20) {
		return
	}

	switch s.id {
	case 13:
		mode := (chip.bit2 ^ chip.bit7) |
			(chip.hithat.bit3 ^ chip.bit5) |
			(chip.topCymbal.bit3 ^ chip.bit5)

		s.phout = uint16(int32(mode) << 9)

		if mode^uint8(s.noiseHH&1) != 0 {
			s.phout |= 0xd0
		} else {
			s.phout |= 0x34
		}
	case 16:
		s.phout = uint16(
			uint32(chip.bit8)<<9 |
				(uint32(chip.bit8)^(chip.noiseSD&1))<<8,
		)

	case 17:
		chip.topCymbal.bit3 = bit(3, phase)
		chip.bit5 = bit(5, phase)

		mode := (chip.bit2 ^ chip.bit7) |
			(chip.hithat.bit3 ^ chip.bit5) |
			(chip.topCymbal.bit3 ^ chip.bit5)

		s.phout = uint16(int32(mode)<<9) | 0x80
	}
}

func (s *slot) updateKSL() {
	var kslShift = [4]uint8{8, 1, 2, 0}

	ksl := (int16(kslROM[s.fic10>>6]) << 2) -
		((8 - int16(s.blk)) << 5)

	ksl = max(0, ksl)

	s.eksl = uint8(ksl)
	s.etks = (uint16(s.rtl) << 2) + (uint16(s.eksl) >> kslShift[s.rksl])
}

func (s *slot) keyOn(kind uint8) {
	s.key |= kind
}

func (s *slot) keyOff(kind uint8) {
	s.key &^= kind
}

type slink blink[int16, slot]

func nilSlink() (nilSlink slink) { return }

func (ml slink) i16() *int16 {
	return ml.dst
}

func (ml slink) from() *slot {
	return ml.src
}

const (
	envGenAttack = iota
	envGenDecay
	envGenSustain
	envGenRelease
)

type envGen struct {
	erout fp123
	eout  fp123
	// einc     uint8
	egen     uint8
	eksl     uint8
	eks      uint8
	etks     uint16
	erates   [4]uint8
	erateHis [4]uint8
	erateLos [4]uint8
}

var envGenSteps = [4][4]uint8{
	{0, 0, 0, 0},
	{1, 0, 0, 0},
	{1, 0, 1, 0},
	{1, 1, 1, 0},
}

type regs struct {
	kind uint8
	rvib uint8
	rksr uint8
	rmul uint8
	rksl uint8
	rtl  uint8
	rar  uint8
	rdr  uint8
	rsl  uint8
	rrr  uint8
	rwf  uint8
}

type phaseGen struct {
	phase uint32
	phinc uint32
	phout uint16
	phrst bool

	phincs [8]uint32
}
