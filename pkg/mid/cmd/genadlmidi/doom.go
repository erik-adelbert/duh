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
	"os"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

type dmxOPL2 struct {
	TremVib0, AtkDec0, SusRel0, Wav0, Sca0, Lvl0 byte
	Feedback                                     byte
	TremVib1, AtkDec1, SusRel1, Wav1, Sca1, Lvl1 byte
	Unused                                       byte
	Note                                         int16
}

type dmxOPL struct {
	Flags          dmxFlags
	Finetune, Note byte
	Patches        [2]dmxOPL2
}

type dmxFlags uint16

const (
	DmxFixedPitched dmxFlags = (1 << iota)
	DmxVibDelay
	Dmx4Op
)

func LoadDoom(banks *mid.BankDB, id int, bi BankInfo) error {
	raw, err := os.ReadFile(bi.FileM)

	if err != nil {
		return err
	}

	bid := banks.AddBank(id, bi.Name, mid.BnkDMX)
	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	for i := range 175 {
		// off0 := 0x18A4 + i*32
		off1 := 8 + i*36
		perc := i >= 128

		var ops [4]mid.OP

		ins := mid.NewInsEntry()

		dmxi := bread[dmxOPL](raw[off1:])

		flags := dmxi.Flags

		var (
			notes [2]int16

			defs = [2]idef{
				silence(),
				silence(),
			}
		)

		for j := range defs {
			defs[j][0] = dmxi.Patches[j].TremVib0
			defs[j][1] = dmxi.Patches[j].TremVib1
			defs[j][2] = dmxi.Patches[j].AtkDec0
			defs[j][3] = dmxi.Patches[j].AtkDec1
			defs[j][4] = dmxi.Patches[j].SusRel0
			defs[j][5] = dmxi.Patches[j].SusRel1
			defs[j][6] = dmxi.Patches[j].Wav0
			defs[j][7] = dmxi.Patches[j].Wav1
			defs[j][8] = dmxi.Patches[j].Sca0 | dmxi.Patches[j].Lvl0
			defs[j][9] = dmxi.Patches[j].Sca1 | dmxi.Patches[j].Lvl1
			defs[j][10] = dmxi.Patches[j].Feedback

			notes[j] = dmxi.Patches[j].Note + 12
			if perc {
				notes[j] = 12
			}

			switch j {
			case 0:
				ins.FBConn = uint16(dmxi.Patches[j].Feedback)
			case 1:
				ins.FBConn |= uint16(dmxi.Patches[j].Feedback) << 8
			}

			toOPs(defs[j], ops[:], 2*j)
		}

		note := dmxi.Note
		if perc && has(flags, DmxFixedPitched) {
			note = 60
		}

		for note > 0 && note < 20 {
			note += 12
			notes[0] -= 12
			notes[1] -= 12
		}

		ins.Note.Off0 = int8(notes[0])
		ins.Note.Off1 = int8(notes[1])

		if has(flags, Dmx4Op) {
			ins.Flags |= mid.Flag4op
		}

		if has(flags, DmxFixedPitched) {
			ins.Flags |= mid.FlagFixedTone
		}

		ins.DrumKey = note
		ins.SecVoice = float64(dmxi.Finetune) - 128

		mb := melo
		patch := i
		if perc {
			mb = drum
			patch = (i - 128) + 35
		}

		banks.AddIns(mb, patch, ins, ops[:], bi.FileM)
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}

func silence() idef {
	return idef{0x00, 0x10, 0x07, 0x07, 0xF7, 0xF7, 0x00, 0x00, 0xFF, 0xFF, 0x00}
}
