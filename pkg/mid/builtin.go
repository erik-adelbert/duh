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
	_ "embed"
	"encoding/gob"
	"fmt"
	"time"
)

//go:generate make -C cmd/genadlmidi build

//go:embed adlmidi.dat
var adlmidi []byte

var (
	builtin *BankDB

	adlIns   []adlData
	adlMetas []adlMeta
	banks    []adlBank
)

func init() {
	builtin = NewBankDB()

	err := gob.NewDecoder(bytes.NewReader(adlmidi)).Decode(builtin)

	if err != nil {
		ErrBuiltin = fmt.Errorf("built-in bank database: %w", err)
		return
	}

	tables, err := builtin.AsTables()
	if err != nil {
		ErrBuiltin = fmt.Errorf("built-in ADL tables: %w", err)
		return
	}

	adlIns = tables.Instruments
	adlMetas = tables.Metas
	banks = tables.Banks
}

type adlData struct {
	ModulatorE862 uint32
	CarrierE862   uint32

	Modulator40 uint8
	Carrier40   uint8

	FeedConn uint8

	Finetune int8
}

type adlMeta struct {
	ID0 uint16
	ID1 uint16

	Tone  uint8
	Flags int

	VelocityOffset int8
	SecondFinetune float64

	KON  time.Duration
	KOFF time.Duration
}

func MkADLMeta() adlMeta {
	return adlMeta{
		Flags: FlagSilence,
	}
}

type adlTables struct {
	Instruments []adlData
	Metas       []adlMeta
	Banks       []adlBank
}

type adlBank struct {
	ID    int
	name  string
	Setup uint16 // low byte: volume model, bit 8: deep tremolo, 9: deep vibrato, 10: MT-32
	Melos []ADLMidiBank
	Drums []ADLMidiBank
}

type ADLMidiBank struct {
	bank
	Programs [128]uint16
}

func BankNames() []string {
	names := make([]string, len(banks))

	for i, bank := range banks {
		names[i] = bank.name
	}

	return names
}

// Melodic4OpCount counts occupied program slots in the first
// melodic MIDI bank of the built-in ADL bank at bankIndex. The four-op count
// excludes pseudo-4-op instruments, which do not use hardware 4-op channels.
// Empty slots are skipped; repeated instruments count once per slot.
// A bank without melodic MIDI banks yields zero counts. Invalid bank indices
// and metadata references are reported as errors.
func Melodic4OpCount(bankIndex int) (fourOp, total uint, err error) {
	if ErrBuiltin != nil {
		return 0, 0, ErrBuiltin
	}
	return default4OpCount(banks, adlMetas, bankIndex)
}

func default4OpCount(banks []adlBank, metas []adlMeta, i int) (fourOp, total uint, err error) {
	if i < 0 || i >= len(banks) {
		ctx := fmt.Sprintf("ADL bank %d is out of range (0..%d)", i, len(banks)-1)
		return 0, 0, mkError(ErrMidi, ctx)
	}

	if len(banks[i].Melos) == 0 {
		return 0, 0, nil
	}

	for prog, id := range banks[i].Melos[0].Programs {
		if id == 0 {
			continue
		}
		if int(id) >= len(metas) {
			ctx := fmt.Sprintf("ADL bank %d melodic program %d references invalid metadata %d", i, prog, id)
			return 0, 0, mkError(ErrMidi, ctx)
		}

		total++
		meta := metas[id]
		if meta.ID0 != meta.ID1 && !IsPseudo4Op(meta.Flags) {
			fourOp++
		}
	}
	return fourOp, total, nil
}
