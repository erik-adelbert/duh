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
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"slices"
	"time"
)

// BankDB represents the database of banks, MIDI banks, instruments, and operators.
type BankDB struct {
	Banks []*BankEntry
	Midis []*MidiBank
	Ins   []*InsEntry
	Ops   []*OP
}

const (
	hintsz     = 128
	midiHintsz = 150
	insHintsz  = 5500
	opHintsz   = 9100
)

// NewBankDB creates a new BankDB with preallocated slices.
func NewBankDB() *BankDB {
	return &BankDB{
		Banks: make([]*BankEntry, 0, hintsz),
		Midis: make([]*MidiBank, 0, midiHintsz),
		Ins:   make([]*InsEntry, 0, insHintsz),
		Ops:   make([]*OP, 0, opHintsz),
	}
}

// Dump serializes the BankDB to the specified output file.
func (db *BankDB) Dump(output string) error {
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close() //nolint:errcheck // can't do anything if it fails

	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(db); err != nil {
		return err
	}

	_, err = out.Write(buf.Bytes())
	return err
}

// OpenDB deserializes a BankDB from the specified input file.
func OpenDB(input string) (*BankDB, error) {
	f, err := os.Open(input)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck // can't do anything if it fails

	dec := gob.NewDecoder(f)

	db := NewBankDB()

	err = dec.Decode(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// AddBank adds or updates a bank entry in the database.
func (db *BankDB) AddBank(ID int, name string, setup uint16) int {
	i := slices.IndexFunc(db.Banks, func(be *BankEntry) bool { return be.ID == ID })

	if i == -1 {
		i = len(db.Banks)
		db.Banks = append(db.Banks, NewBankEntry(ID))
	}

	db.Banks[i].Name = name
	db.Banks[i].Setup = setup

	return i
}

// AddMidi adds or updates a MIDI bank in the database.
func (db *BankDB) AddMidi(ID int, mb *MidiBank, drum bool) {
	be := db.Banks[ID]

	i := slices.IndexFunc(db.Midis, func(x *MidiBank) bool {
		return x.LSB == mb.LSB && x.MSB == mb.MSB && x.Ins == mb.Ins
	})

	if i == -1 {
		i = len(db.Midis)
		db.Midis = append(db.Midis, mb)
	}

	if drum {
		be.Drums = append(be.Drums, i)
	} else {
		be.Melos = append(be.Melos, i)
	}
}

// AddIns registers an instrument in mb at patch ID. Like the C++ reference,
// the entry is taken by value: callers may keep mutating *src afterwards.
func (db *BankDB) AddIns(mb *MidiBank, ID int, ie *InsEntry, op []OP, meta string) {
	nop := 2
	if Is4Op(ie.Flags) {
		nop = 4
	}

	if has(ie.Flags, FlagSilence) {
		mb.Ins[ID] = -1
		return
	}

	for i := range nop {
		o := op[i]

		j := slices.IndexFunc(db.Ops, func(x *OP) bool { return *x == o })
		if j == -1 {
			j = len(db.Ops)
			db.Ops = append(db.Ops, &o)
		}

		ie.Ops[i] = j
	}

	meta = fmt.Sprintf("%s_%d", meta, ID)

	i := slices.IndexFunc(db.Ins, func(x *InsEntry) bool { return x.eq(ie) })

	if i == -1 {
		i = len(db.Ins)
		db.Ins = append(db.Ins, ie)
	}

	db.Ins[i].Metas = append(db.Ins[i].Metas, meta)

	mb.Ins[ID] = i
}

// eq compares instrument content, ignoring the Metas field.
func (ie *InsEntry) eq(o *InsEntry) bool {
	return ie.InsData == o.InsData
}

// BankEntry represents a bank of instruments, either melodic or drum.
type BankEntry struct {
	ID     int
	Name   string
	Flags  int
	Volmod int
	Setup  uint16

	Melos []int
	Drums []int
}

// NewBankEntry creates a new BankEntry with the given ID and preallocated slices for melodic and drum instruments.
func NewBankEntry(id int) *BankEntry {
	return &BankEntry{
		ID: id,

		Melos: make([]int, 0, hintsz),
		Drums: make([]int, 0, hintsz),
	}
}

// MidiBank represents a MIDI bank with LSB, MSB, and instrument mappings.
type MidiBank struct {
	LSB, MSB byte
	Ins      [128]int
}

// NewMidiBank creates a new MIDI bank with the specified LSB and MSB, initializing all instrument slots to silence (-1).
func NewMidiBank(lsb, msb uint8) *MidiBank {
	mb := &MidiBank{
		LSB: lsb,
		MSB: msb,
	}

	for i := range mb.Ins {
		mb.Ins[i] = -1
	}

	return mb
}

// InsEntry represents an instrument entry with metadata and instrument data.
type InsEntry struct {
	Metas []string

	InsData
}

// InsData represents the detailed data of an instrument, including note offsets, velocity offsets, flags, feedback/connection, and operator indices.
type InsData struct {
	Note struct {
		Off0, Off1 int8
	}

	VelOff   int8
	SecVoice float64
	DrumKey  uint8

	Flags int

	FBConn uint16

	KON, KOFF        time.Duration
	HasStoredTimings bool

	Ops [4]int
}

// NewInsEntry creates a new InsEntry with an empty Metas slice.
func NewInsEntry() *InsEntry {
	return &InsEntry{
		Metas: []string{},
	}
}

// SetFBConn sets the feedback/connection for an InsEntry.
func (ie *InsEntry) SetFBConn(fbcon0, fbcon1 uint16) {
	ie.FBConn = (fbcon0 & 0xF) | ((fbcon1 & 0xF) << 8)
}

// Is4Op checks if the given flags indicate a 4-operator instrument.
func Is4Op(flags int) bool {
	return has(flags, Flag4op|FlagPseudo4op)
}

// IsPseudo4Op checks if the given flags indicate a pseudo 4-operator instrument.
func IsPseudo4Op(flags int) bool {
	return has(flags, FlagPseudo4op)
}

// OP represents the operator registers for an instrument.
// E862 packs registers 0xE0, 0x80, 0x60 and 0x20; D40 is register 0x40.
// D40 packs register 0x40.
type OP struct {
	// E862 packs registers 0xE0, 0x80, 0x60 and 0x20; D40 is register 0x40.
	E862 uint32
	D40  uint32
}

const (
	FlagDeepTremolo = 1 << iota
	FlagDeepVibrato
	FlagMT32
)

const (
	VolLinear = iota
	VolNative
	VolDMX
	VolApogee
	VolWin9x
	VolDMXFixed
	VolApogeeFixed
	VolAIL
	VolWin9xFM
	VolHMI
	VolHMIOld
	VolMSAdlib
	VolIMFCreator
	VolOConnel
	VolModelCount
)

const (
	BnkGeneric = 0x300
	BnkWin9x   = 0x304 //SB16
	BnkWin9xFM = 0x308 //Generic FM
	BnkDMX     = 0x002
	BnkApogee  = 0x003
	BnkAIL     = 0x307
	BnkAILMT32 = 0x707
	BnkIBK     = 0x30D
	BnkIBKMT32 = 0x70D
	BnkIMF     = 0x200
	BnkCMF     = 0x201
	BnkHMI     = 0x309
	BnkHMIOld  = 0x30A
)

const (
	DrumBass   = 0x08
	DrumSnare  = 0x10
	DrumTomTom = 0x18
	DrumCymbal = 0x20
	DrumHiHat  = 0x28
)
