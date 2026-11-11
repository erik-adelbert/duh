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

import "fmt"

// AsTables converts the bank database into the instrument, meta, and patch
// lookup tables consumed by the MIDI decoder.
func (db *BankDB) AsTables() (*adlTables, error) {
	if db == nil {
		return nil, fmt.Errorf("derive ADL tables: nil BankDB")
	}

	tables := new(adlTables{
		Instruments: []adlData{{}}, // First entry is silence
		Metas:       []adlMeta{MkADLMeta()},
		Banks:       make([]adlBank, len(db.Banks)),
	})

	metas := make([]int, len(db.Ins))

	for i, ins := range db.Ins {
		if ins == nil {
			ctx := fmt.Sprintf("nil instrument %d", i)
			return nil, mkError(ErrDB, ctx)
		}

		if has(ins.Flags, FlagSilence) {
			continue
		}

		npair := 1
		if Is4Op(ins.Flags) {
			npair = 2
		}

		meta := adlMeta{
			ID0:            0,
			ID1:            0,
			VelocityOffset: ins.VelOff,
			SecondFinetune: ins.SecVoice,
			KON:            ins.KON,
			KOFF:           ins.KOFF,
		}

		if has(ins.Flags, FlagFixedTone) {
			meta.Tone = ins.DrumKey
		}

		meta.Flags |= ins.Flags & (FlagRhythmMask | FlagPseudo4op | Flag4op)

		for j := range npair {
			mi := ins.Ops[j*2]
			ci := ins.Ops[j*2+1]

			if mi < 0 || mi >= len(db.Ops) {
				ctx := fmt.Sprintf("instrument %d invalid operator %d", i, mi)
				return nil, mkError(ErrDB, ctx)
			}

			if ci < 0 || ci >= len(db.Ops) {
				ctx := fmt.Sprintf("instrument %d invalid operator %d", i, ci)
				return nil, mkError(ErrDB, ctx)
			}

			mod, car := db.Ops[mi], db.Ops[ci]

			if mod == nil || car == nil {
				ctx := fmt.Sprintf("instrument %d nil operator", i)
				return nil, mkError(ErrDB, ctx)
			}

			ft := ins.Note.Off1
			if j == 0 {
				ft = ins.Note.Off0
			}

			data := adlData{
				ModulatorE862: mod.E862,
				CarrierE862:   car.E862,
				Modulator40:   uint8(mod.D40),
				Carrier40:     uint8(car.D40),
				FeedConn:      uint8(ins.FBConn >> (8 * j)),
				Finetune:      ft,
			}

			id := uint16(len(tables.Instruments))
			tables.Instruments = append(tables.Instruments, data)

			if j == 0 {
				meta.ID0 = id
			}

			meta.ID1 = id
		}

		metas[i] = len(tables.Metas)
		tables.Metas = append(tables.Metas, meta)
	}

	hasMappings := false
	for i, bank := range db.Banks {
		if bank == nil {
			ctx := fmt.Sprintf("nil bank at index %d", i)
			return nil, mkError(ErrDB, ctx)
		}

		hasMappings = hasMappings || len(bank.Melos)+len(bank.Drums) != 0

		out := adlBank{
			ID:    bank.ID,
			name:  bank.Name,
			Setup: bank.Setup,
			// Preserve bank-select variants and their melodic/percussion roles.
			Melos: make([]ADLMidiBank, 0, len(bank.Melos)),
			Drums: make([]ADLMidiBank, 0, len(bank.Drums)),
		}

		for _, j := range bank.Melos {
			mb, err := db.mkMidiBank(j, metas)
			if err != nil {
				ctx := fmt.Sprintf("bank %d melodic bank: %v", i, err)
				return nil, mkError(ErrDB, ctx)
			}

			out.Melos = append(out.Melos, mb)
		}

		for _, j := range bank.Drums {
			mb, err := db.mkMidiBank(j, metas)
			if err != nil {
				ctx := fmt.Sprintf("bank %d drum bank: %v", i, err)
				return nil, mkError(ErrDB, ctx)
			}

			out.Drums = append(out.Drums, mb)
		}

		tables.Banks[i] = out
	}

	if len(db.Banks) != 0 && !hasMappings {
		ctx := "no MIDI bank mappings; regenerate the database"
		return nil, mkError(ErrDB, ctx)
	}

	return tables, nil
}

func (db *BankDB) mkMidiBank(i int, metas []int) (ADLMidiBank, error) {
	if i < 0 || i >= len(db.Midis) {
		ctx := fmt.Sprintf("invalid MIDI bank %d", i)
		return ADLMidiBank{}, mkError(ErrDB, ctx)
	}

	bank := db.Midis[i]
	if bank == nil {
		ctx := fmt.Sprintf("nil MIDI bank %d", i)
		return ADLMidiBank{}, mkError(ErrDB, ctx)
	}

	mb := ADLMidiBank{LSB: bank.LSB, MSB: bank.MSB}
	for i, j := range bank.Ins {
		if j == -1 {
			continue
		}

		if j >= len(metas) {
			ctx := fmt.Sprintf("patch %d invalid instrument %d", i, j)
			return ADLMidiBank{}, mkError(ErrDB, ctx)
		}

		mb.Programs[i] = uint16(metas[j])
	}
	return mb, nil
}
