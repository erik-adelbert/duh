// duh ADL midi driver for OPL3
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

func LoadJunglevision(banks *mid.BankDB, id int, bi BankInfo) error {
	raw, err := os.ReadFile(bi.FileM)

	if err != nil {
		return err
	}

	bid := banks.AddBank(id, bi.Name, mid.BnkWin9x)
	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	nmelo := u16(raw[0x20:])
	ndrum := u16(raw[0x22:])

	melo0 := u16(raw[0x24:])
	drum0 := u16(raw[0x26:])

	for i := range nmelo + ndrum {
		perc := i >= nmelo

		off0 := 0x28 + i*0x18
		off1 := off0 + 11

		var (
			defs [2]idef
			ops  [4]mid.OP
		)

		defs[0][0] = raw[off0+2]
		defs[0][1] = raw[off0+8]
		defs[0][2] = raw[off0+4]
		defs[0][3] = raw[off0+10]
		defs[0][4] = raw[off0+5]
		defs[0][5] = raw[off0+11]
		defs[0][6] = raw[off0+6]
		defs[0][7] = raw[off0+12]
		defs[0][8] = raw[off0+3]
		defs[0][9] = raw[off0+9]
		defs[0][10] = raw[off0+7] &^ 0x30

		defs[1][0] = raw[off1+2]
		defs[1][1] = raw[off1+8]
		defs[1][2] = raw[off1+4]
		defs[1][3] = raw[off1+10]
		defs[1][4] = raw[off1+5]
		defs[1][5] = raw[off1+11]
		defs[1][6] = raw[off1+6]
		defs[1][7] = raw[off1+12]
		defs[1][8] = raw[off1+3]
		defs[1][9] = raw[off1+9]
		defs[1][10] = raw[off1+7] &^ 0x30

		ins := mid.NewInsEntry()

		note := raw[off0+1]
		for note > 0 && note < 20 {
			note += 12

			ins.Note.Off0 -= 12
			ins.Note.Off1 -= 12
		}

		if raw[off0] != 0 {
			ins.Flags |= mid.Flag4op
		}

		ins.DrumKey = note
		ins.SetFBConn(uint16(raw[off0+7]), uint16(raw[off1+7]))

		toOPs(defs[0], ops[:], 0)
		toOPs(defs[1], ops[:], 2)

		mb := melo
		patch := int(i + melo0)
		if perc {
			mb = drum
			patch = int(i + drum0 - melo0)
		}

		banks.AddIns(mb, patch, ins, ops[:], bi.FileM)
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}
