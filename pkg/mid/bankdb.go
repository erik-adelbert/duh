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

type BankDB struct {
	Banks []*BankEntry
	Midis []*MidiBank
	Ins   []*InsEntry
	Ops   []*OP
}

const (
	Hintsz     = 128
	MidiHintsz = 150
	InsHintsz  = 5500
	OpHintsz   = 9100
)

func NewBankDB() *BankDB {
	return &BankDB{
		Banks: make([]*BankEntry, 0, Hintsz),
		Midis: make([]*MidiBank, 0, MidiHintsz),
		Ins:   make([]*InsEntry, 0, InsHintsz),
		Ops:   make([]*OP, 0, OpHintsz),
	}
}

func (db *BankDB) Dump(output string) error {
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	var buf bytes.Buffer

	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(db); err != nil {
		return err
	}

	_, err = out.Write(buf.Bytes())
	return err
}

func OpenDB(input string) (*BankDB, error) {
	f, err := os.Open(input)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	dec := gob.NewDecoder(f)

	db := NewBankDB()

	err = dec.Decode(db)
	if err != nil {
		return nil, err
	}

	return db, nil
}

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

type BankEntry struct {
	ID     int
	Name   string
	Flags  int
	Volmod int
	Setup  uint16

	Melos []int
	Drums []int
}

func NewBankEntry(id int) *BankEntry {
	return &BankEntry{
		ID: id,

		Melos: make([]int, 0, Hintsz),
		Drums: make([]int, 0, Hintsz),
	}
}

type MidiBank struct {
	LSB, MSB byte
	Ins      [128]int
}

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

type InsEntry struct {
	Metas []string

	InsData
}

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

func NewInsEntry() *InsEntry {
	return &InsEntry{
		Metas: []string{},
	}
}

func (ie *InsEntry) SetFBConn(fbcon0, fbcon1 uint16) {
	ie.FBConn = (fbcon0 & 0xF) | ((fbcon1 & 0xF) << 8)
}

func Is4Op(flags int) bool {
	return has(flags, Flag4op|FlagPseudo4op)
}

func IsPseudo4Op(flags int) bool {
	return has(flags, FlagPseudo4op)
}

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
