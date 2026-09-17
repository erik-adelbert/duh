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

type Registers [2][256]uint8

type OPL3Flag = uint16

const (
	CSM OPL3Flag = iota
	WF
	HH
	CYM
	TOM
	SD
	BD
	DRUM
	VIBI
	AMI
	OPL3
	FOUROP
	NOPL3FLAG
)

type Flags uint16

func (f Flags) Has(i int) bool {
	return f&(1<<i) != 0
}

func (regs Registers) Status() Flags {
	var u16 uint16

	var fourOp uint16
	if regs[Bank1][0x04] > 0 {
		fourOp = 1
	}

	u16 |= uint16(bit(7, regs[Bank0][0x01])) << CSM
	u16 |= uint16(bit(5, regs[Bank0][0x01])) << WF

	u16 |= uint16(regs[Bank0][0xbd]) << HH // get all drum at once

	u16 |= uint16(bit(1, regs[Bank1][0x05])) << OPL3
	u16 |= uint16(fourOp << FOUROP)

	return Flags(u16)
}

func (regs Registers) Channel(id int) (ch OPL3Channel) {
	if uint(id) > 8 {
		return
	}

	ch.ID = id

	ops := [2]uint8{
		opMapping[(id<<1)+0],
		opMapping[(id<<1)+1],
	}

	for bank, o := range ops {
		b0 := regs[bank][0xb0+id]

		fq := uint16(lo2(b0)) << 8
		fq |= uint16(regs[bank][0xa0+id])

		oct := lo3(b0 >> 2)

		c0 := regs[bank][0xc0+id]

		fb := lo3(c0 >> 1)

		var flags OpFlags

		if bit(0, regs[bank][0xc0+id]) == 1 {
			flags |= (1 << ALG)
		}

		if bit(5, b0) == 1 {
			flags |= (1 << KEY)
			flags |= OpFlags(hi4(regs[bank][0x20+o]))
		}

		ch.Ops[bank] = OP{
			OpFlags: flags,

			OpState: [9]uint8{
				ML:  lo4(regs[bank][0x20+o]),
				K:   hi2(regs[bank][0x40+o]),
				TL:  lo6(regs[bank][0x40+o]),
				A:   hi4(regs[bank][0x60+o]),
				D:   lo4(regs[bank][0x60+o]),
				S:   hi4(regs[bank][0x80+o]),
				R:   lo4(regs[bank][0x80+o]),
				OCT: oct,
				FB:  fb,
			},

			FQ: fq,

			WF: Waveform(lo3(regs[bank][0xe0+o])),
		}
	}

	return
}

type OPL3Channel struct {
	ID  int
	Ops [2]OP
}

type OP struct {
	OpFlags
	OpState
	FQ uint16
	WF Waveform
}

type OpFlags uint8

const (
	KSR OpFlags = iota
	EGT
	VIB
	AM
	KEY
	ALG
)

func (f OpFlags) Has(flag OpFlags) bool {
	return f&(1<<flag) != 0
}

type OpState = [9]uint8

const (
	ML = iota
	K
	TL
	A
	D
	S
	R
	OCT
	FB
)

type Waveform uint8

const (
	Wave0 Waveform = iota
	Wave1
	Wave2
	Wave3
	Wave4
	Wave5
	Wave6
	Wave7
)

func (w Waveform) String() string {
	w8 := uint8(w)

	return []string{
		Wave0: "^v^v",
		Wave1: "^-^-",
		Wave2: "^^^^",
		Wave3: "/|/|",
		Wave4: "^v--",
		Wave5: "^^--",
		Wave6: "_-_-",
		Wave7: "////",
	}[lo3(w8)] // only 3 bits are valid
}

var opMapping = [18]uint8{
	0x00, 0x03, 0x01, 0x04, 0x02, 0x05,
	0x08, 0x0b, 0x09, 0x0c, 0x0a, 0x0d,
	0x10, 0x13, 0x11, 0x14, 0x12, 0x15,
}
