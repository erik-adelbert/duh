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
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

const (
	WOPLFlagDeepTremolo = 1 << iota
	WOPLFileDeepVibrato
	WOPLFileMT32
)

const (
	WOPLIns2Op       = 0x00
	WOPLIns4Op       = 0x01
	WOPLInsPseudo4Op = 0x02
	WOPLInsBlank     = 0x04
	WOPLInsRythmMode = 0x38
	WOPLInsFIxedNote = 0x40
	WOPLInsMask      = 0x47
)

type WOPLBank struct {
	name     string
	lsb, msb uint8
	ins      [128]WOPLIns
}

// Name returns the bank name as a string
func (b *WOPLBank) Name() string {
	return strings.TrimRight(b.name, "\x00")
}

// SetName sets the bank name (max 32 chars)
func (b *WOPLBank) SetName(name string) {
	b.name = name
}

type WOPLIFile struct {
	ver  uint16
	drum bool
	ins  WOPLIns
}

// LoadInsFromMem parses a single-instrument file from memory
func LoadInsFromMem(mem []byte) (*WOPLIFile, error) {
	if mem == nil {
		return nil, errNullPointer
	}

	cursor := 0
	length := len(mem)

	advance := func(n int) {
		cursor += n
		length -= n
	}

	// Magic number
	if length < 11 {
		return nil, errUnexpectedEnding
	}
	if string(mem[cursor:cursor+11]) != WOPLINS {
		return nil, errBadMagic
	}

	advance(11)

	// Version
	if length < 2 {
		return nil, errUnexpectedEnding
	}

	ver := u16(mem[cursor : cursor+2])
	if ver == 0 {
		return nil, errInvalidVersion
	}

	if ver > WOPLVER {
		return nil, errNewerVersion
	}

	advance(2)

	// Is drum flag
	if length < 1 {
		return nil, errUnexpectedEnding
	}

	var drum bool

	if mem[cursor] != 0 {
		drum = true
	}

	advance(1)

	var inssz int
	if ver > 2 {
		// Sounding delays are not part of single-instrument file
		inssz = WOPL3INSSZ - 4
	} else {
		inssz = WOPL2INSSZ
	}

	if length < inssz {
		return nil, errUnexpectedEnding
	}

	ins := ParseInstrument(mem[cursor:], ver, false)

	return &WOPLIFile{
		ver:  ver,
		drum: drum,
		ins:  ins,
	}, nil
}

// SaveInstToMem writes a single-instrument file to the destination byte slice
func SaveInsToMem(file *WOPLIFile, dest []byte, version uint16) error {
	if file == nil {
		return errNullPointer
	}
	if version == 0 {
		version = WOPLVER
	}

	cursor := 0
	length := len(dest)

	advance := func(n int) {
		cursor += n
		length -= n
	}

	// Magic
	if length < 11 {
		return errUnexpectedEnding
	}

	copy(dest[cursor:cursor+11], []byte(WOPLINS))

	advance(11)

	// Version
	if length < 2 {
		return errUnexpectedEnding
	}
	binary.LittleEndian.PutUint16(dest[cursor:cursor+2], version)
	advance(2)

	// Is drum flag
	if length < 1 {
		return errUnexpectedEnding
	}

	dest[cursor] = 0
	if file.drum {
		dest[cursor] = 1
	}

	advance(1)

	var inssz int
	if version > 2 {
		inssz = WOPL3INSSZ - 4
	} else {
		inssz = WOPL2INSSZ
	}

	if length < inssz {
		return errUnexpectedEnding
	}

	WriteInstrument(&file.ins, dest[cursor:], version, false)

	return nil
}

// CalculateInsFileSize returns the size needed for a single-instrument file
func CalculateInsFileSize(file *WOPLIFile, version uint16) int {
	if file == nil {
		return 0
	}

	if version == 0 {
		version = WOPLVER
	}

	filesz := 11 + 2 + 1

	var inssz int

	if version > 2 {
		inssz = WOPL3INSSZ - 4
	} else {
		inssz = WOPL2INSSZ
	}

	filesz += inssz

	return filesz
}

const (
	WOPLOPCar0 = iota
	WOPLOPMod0
	WOPLOPCar1
	WOPLOPMod1
)

type WOPLOp struct {
	avek20 uint8
	ksll40 uint8
	adec60 uint8
	susr80 uint8
	wave80 uint8
}

const (
	WOPLMAGIC = "WOPL3-BANK\x00"
	WOPLINS   = "WOPL3-INST\x00"
	WOPLVER   = 3

	WOPL3INSSZ = 66
	WOPL2INSSZ = 62
)

type WOPLFile struct {
	ver    uint16
	flags  uint8
	volmod uint8

	melos []WOPLBank
	drums []WOPLBank
}

// WOPL_Init initializes a WOPLFile with the given number of melodic and
// percussive banks. A value of 0 means "create one blank bank".
func NewWOPLFile(version uint16, flags, volmod uint8, nmelo, ndrum uint16) *WOPLFile {
	file := &WOPLFile{
		ver:    version,
		flags:  flags,
		volmod: volmod,
		melos:  make([]WOPLBank, max(1, nmelo)),
		drums:  make([]WOPLBank, max(1, ndrum)),
	}

	if nmelo == 0 {
		for i := range file.melos[0].ins {
			file.melos[0].ins[i].flags = WOPLInsBlank
		}
	}

	if ndrum == 0 {
		for i := range file.drums[0].ins {
			file.drums[0].ins[i].flags = WOPLInsBlank
		}
	}

	return file
}

// LoadBankFromMem parses a WOPL bank file from memory
func LoadBankFromMem(mem []byte) (*WOPLFile, error) {
	if mem == nil {
		return nil, errNullPointer
	}

	cursor := 0
	length := len(mem)

	advance := func(n int) {
		cursor += n
		length -= n
	}

	// Magic number
	if length < 11 {
		return nil, errUnexpectedEnding
	}

	if string(mem[cursor:cursor+11]) != WOPLMAGIC {
		return nil, errBadMagic
	}

	advance(11)

	// Version
	if length < 2 {
		return nil, errUnexpectedEnding
	}

	ver := u16(mem[cursor : cursor+2])

	if ver == 0 {
		return nil, errInvalidVersion
	}

	if ver > WOPLVER {
		return nil, errNewerVersion
	}

	advance(2)

	// Header
	if length < 6 {
		return nil, errUnexpectedEnding
	}

	nmelo := U16(mem[cursor : cursor+2])
	ndrum := U16(mem[cursor+2 : cursor+4])

	flags := mem[cursor+4]
	volmod := mem[cursor+5]

	advance(6)

	out := NewWOPLFile(ver, flags, volmod, nmelo, ndrum)

	// Bank names and MIDI LSB/MSB (version >= 2)
	if ver >= 2 {
		for slot := range 2 {
			banks := out.drums
			count := ndrum

			if slot == 0 {
				banks = out.melos
				count = nmelo
			}

			for j := range count {
				if length < 34 {
					return nil, errUnexpectedEnding
				}

				banks[j].name = string(mem[cursor : cursor+32])

				banks[j].lsb = mem[cursor+32]
				banks[j].msb = mem[cursor+33]

				advance(34)
			}
		}
	}

	// Instruments
	inssz := WOPL2INSSZ
	if ver > 2 {
		inssz = WOPL3INSSZ
	}

	for slot := range 2 {
		banks := out.drums
		count := ndrum

		if slot == 0 {
			banks = out.melos
			count = nmelo
		}

		if length < 128*inssz*int(count) {
			return nil, errUnexpectedEnding
		}

		for j := range count {
			for k := range 128 {
				banks[j].ins[k] = ParseInstrument(mem[cursor:], ver, true)

				advance(inssz)
			}
		}
	}

	return out, nil
}

// SaveBankToMem writes a bank file to the destination byte slice
func SaveBankToMem(file *WOPLFile, dest []byte, version uint16, forceGM bool) error {
	if file == nil {
		return errNullPointer
	}

	if version == 0 {
		version = WOPLVER
	}

	cursor := 0
	length := len(dest)

	advance := func(n int) {
		cursor += n
		length -= n
	}

	nmelo := uint16(len(file.melos))
	ndrum := uint16(len(file.drums))

	if forceGM {
		nmelo = 1
		ndrum = 1
	}

	// Magic
	if length < 11 {
		return errUnexpectedEnding
	}

	copy(dest[cursor:cursor+11], []byte(WOPLMAGIC))

	advance(11)

	// Version
	if length < 2 {
		return errUnexpectedEnding
	}

	binary.LittleEndian.PutUint16(dest[cursor:cursor+2], version)
	advance(2)

	// Melodic bank count
	if length < 2 {
		return errUnexpectedEnding
	}

	binary.BigEndian.PutUint16(dest[cursor:cursor+2], nmelo)
	advance(2)

	// Percussive bank count
	if length < 2 {
		return errUnexpectedEnding
	}

	binary.BigEndian.PutUint16(dest[cursor:cursor+2], ndrum)
	advance(2)

	// Flags
	if length < 2 {
		return errUnexpectedEnding
	}
	dest[cursor] = file.flags
	dest[cursor+1] = file.volmod

	advance(2)

	// Bank metadata (version >= 2)
	if version >= 2 {
		for slot := range 2 {
			banks := file.drums
			count := ndrum

			if slot == 0 {
				banks = file.melos
				count = nmelo
			}

			for j := range count {
				if length < 34 {
					return errUnexpectedEnding
				}

				copy(dest[cursor:cursor+32], banks[j].name)

				dest[cursor+32] = banks[j].lsb
				dest[cursor+33] = banks[j].msb

				advance(34)
			}
		}
	}

	// Instruments
	inssz := WOPL2INSSZ
	if version > 2 {
		inssz = WOPL3INSSZ
	}

	for slot := range 2 {
		banks := file.drums
		count := ndrum

		if slot == 0 {
			banks = file.melos
			count = nmelo
		}

		if length < 128*inssz*int(count) {
			return errUnexpectedEnding
		}

		for j := range count {
			for k := range 128 {
				WriteInstrument(&banks[j].ins[k], dest[cursor:], version, true)
				advance(inssz)
			}
		}
	}

	return nil
}

// CalculateBankFileSize returns the size needed for a bank file
func CalculateBankFileSize(file *WOPLFile, ver uint16) int {
	if file == nil {
		return 0
	}

	if ver == 0 {
		ver = WOPLVER
	}

	banksz := 11 + 2 + 2 + 2 + 1 + 1

	if ver >= 2 {
		banksz += (32 + 1 + 1) * len(file.melos)
		banksz += (32 + 1 + 1) * len(file.drums)
	}

	inssz := WOPL2INSSZ
	if ver > 2 {
		inssz = WOPL3INSSZ
	}

	banksz += (inssz * 128) * len(file.melos)
	banksz += (inssz * 128) * len(file.drums)

	return banksz
}

type WOPLIns struct {
	name string
	note struct {
		off0, off1 int16
	}
	veloff int8
	secvod int8
	drumid uint8
	flags  uint8

	fbc0 struct {
		con0, con1 uint8
	}

	ops [4]WOPLOp

	kon, koff time.Duration
}

// ParseInstrument reads an instrument from the byte slice at the given offset
func ParseInstrument(data []byte, version uint16, timed bool) WOPLIns {
	var ins WOPLIns

	ins.name = string(data[0:32])

	ins.note.off0 = int16(U16(data[32:34]))
	ins.note.off1 = int16(U16(data[34:36]))

	ins.veloff = int8(data[36])
	ins.secvod = int8(data[37])
	ins.drumid = data[38]

	ins.flags = data[39]

	ins.fbc0.con0 = data[40]
	ins.fbc0.con1 = data[41]

	for i := range 4 {
		off := 42 + i*5
		ins.ops[i].avek20 = data[off+0]
		ins.ops[i].ksll40 = data[off+1]
		ins.ops[i].adec60 = data[off+2]
		ins.ops[i].susr80 = data[off+3]
		ins.ops[i].wave80 = data[off+4]
	}

	if version > 2 && timed {
		ins.kon = time.Duration(U16(data[62:64])) * time.Millisecond
		ins.koff = time.Duration(U16(data[64:66])) * time.Millisecond
	}

	return ins
}

// WriteInstrument writes an instrument to the byte slice at the given offset
func WriteInstrument(ins *WOPLIns, data []byte, version uint16, hasSoundingDelays bool) {
	copy(data[0:32], ins.name)

	binary.BigEndian.PutUint16(data[32:34], uint16(ins.note.off0))
	binary.BigEndian.PutUint16(data[34:36], uint16(ins.note.off1))

	data[36] = uint8(ins.veloff)
	data[37] = uint8(ins.secvod)
	data[38] = ins.drumid
	data[39] = ins.flags
	data[40] = ins.fbc0.con0
	data[41] = ins.fbc0.con1

	for i := range 4 {
		off := 42 + i*5
		data[off+0] = ins.ops[i].avek20
		data[off+1] = ins.ops[i].ksll40
		data[off+2] = ins.ops[i].adec60
		data[off+3] = ins.ops[i].susr80
		data[off+4] = ins.ops[i].wave80
	}

	if version > 2 && hasSoundingDelays {
		binary.BigEndian.PutUint16(data[62:64], uint16(ins.kon/time.Millisecond))
		binary.BigEndian.PutUint16(data[64:66], uint16(ins.koff/time.Millisecond))
	}
}

// type BankID struct {
// 	_, _ uint8
// }

func LoadWOPL(banks *mid.BankDB, id int, bi BankInfo) error {
	fn := bi.FileM
	raw, err := os.ReadFile(fn)

	if err != nil {
		return err
	}

	wopl, err := LoadBankFromMem(raw)
	if err != nil {
		return fmt.Errorf("parse WOPL: %w", err)
	}
	if wopl.volmod > mid.VolOConnel {
		return fmt.Errorf("parse WOPL: unsupported volume model %d", wopl.volmod)
	}

	load := func(mb *mid.MidiBank, patch int, win WOPLIns, perc bool) {
		if has(win.flags, WOPLInsBlank) {
			return
		}

		var (
			defs [2]idef
			ops  [4]mid.OP
		)

		ins := mid.NewInsEntry()

		defs[0] = mkdef(win.ops[WOPLOPMod0], win.ops[WOPLOPCar0], win.fbc0.con0)

		quad := has(win.flags, WOPLIns4Op|WOPLInsPseudo4Op)

		defs[1] = defs[0]
		if quad {
			defs[1] = mkdef(win.ops[WOPLOPMod1], win.ops[WOPLOPCar1], win.fbc0.con1)
			ins.Note.Off1 = int8(win.note.off1)

			if has(win.flags, WOPLInsPseudo4Op) {
				ins.Flags |= mid.FlagPseudo4op
			} else {
				ins.Flags |= mid.Flag4op
			}
		}

		if wopl.ver >= 3 {
			ins.KON = win.kon
			ins.KOFF = win.koff
			ins.HasStoredTimings = true
		}

		ins.Flags |= int(win.flags) &^ (WOPLIns4Op | WOPLInsPseudo4Op)

		ins.Note.Off0 = int8(win.note.off0)
		ins.DrumKey = win.drumid
		if perc && win.drumid != 0 {
			ins.Flags |= mid.FlagFixedTone
		}
		ins.SecVoice = float64(((int(win.secvod)+128)>>1)-64) / 32
		ins.VelOff = win.veloff

		ins.SetFBConn(uint16(win.fbc0.con0), uint16(win.fbc0.con1))

		toOPs(defs[0], ops[:], 0)
		toOPs(defs[1], ops[:], 2)

		banks.AddIns(mb, patch, ins, ops[:], fn)
	}

	bid := banks.AddBank(id, bi.Name, uint16(wopl.volmod)|uint16(wopl.flags&7)<<8)

	reg := func(wbs []WOPLBank, perc bool) error {
		for _, wb := range wbs {
			mb := mid.NewMidiBank(wb.lsb, wb.msb)

			for patch, win := range wb.ins {
				load(mb, patch, win, perc)
			}

			banks.AddMidi(bid, mb, perc)
		}

		return nil
	}

	if err = reg(wopl.melos, false); err != nil {
		return err
	}

	if err = reg(wopl.drums, true); err != nil {
		return err
	}

	return nil
}

func mkdef(modulator, carrier WOPLOp, feedback uint8) idef {
	return idef{
		carrier.avek20, modulator.avek20,
		carrier.adec60, modulator.adec60,
		carrier.susr80, modulator.susr80,
		carrier.wave80, modulator.wave80,
		carrier.ksll40, modulator.ksll40,
		feedback,
	}
}

// Name returns the instrument name as a string
func (i *WOPLIns) Name() string {
	return strings.TrimRight(i.name, "\x00")
}

// SetName sets the instrument name (max 32 chars)
func (i *WOPLIns) SetName(name string) {
	i.name = name
}

func U16(data []byte) uint16 {
	return binary.BigEndian.Uint16(data)
}

func U32(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}

var (
	errNullPointer      = errors.New("null pointer")
	errUnexpectedEnding = errors.New("unexpected ending")
	errBadMagic         = errors.New("bad magic")
	errInvalidVersion   = errors.New("invalid version")
	errNewerVersion     = errors.New("newer version")
)
