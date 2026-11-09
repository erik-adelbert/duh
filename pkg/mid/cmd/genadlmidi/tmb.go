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

func LoadTMB(banks *mid.BankDB, id int, bi BankInfo) error {
	perc := bi.FileD != ""

	fn := bi.FileM
	if perc {
		fn = bi.FileD
	}

	raw, err := os.ReadFile(fn)

	if err != nil {
		return err
	}

	bid := banks.AddBank(id, bi.Name, mid.BnkApogee)
	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	patch := 0
	for i := range 256 {
		off0 := i * 0x0D

		var (
			def idef
			ops [4]mid.OP
		)

		def[0] = raw[off0+0]
		def[1] = raw[off0+1]
		def[2] = raw[off0+4]
		def[3] = raw[off0+5]
		def[4] = raw[off0+6]
		def[5] = raw[off0+7]
		def[6] = raw[off0+8]
		def[7] = raw[off0+9]
		def[8] = raw[off0+2]
		def[9] = raw[off0+3]
		def[10] = raw[off0+10]

		ins := mid.NewInsEntry()

		ins.DrumKey = raw[off0+11]
		ins.VelOff = int8(raw[off0+12])
		ins.FBConn = uint16(raw[off0+10])

		toOPs(def, ops[:], 0)

		mb := melo
		if perc {
			mb = drum
		}

		banks.AddIns(mb, patch, ins, ops[:], fn)

		patch++
		if patch == 128 {
			patch = 0
		}
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}
