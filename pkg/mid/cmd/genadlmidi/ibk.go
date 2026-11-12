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

func LoadIBK(banks *mid.BankDB, id int, bi BankInfo) error {
	perc := bi.FileD != ""

	fn := bi.FileM
	if perc {
		fn = bi.FileD
	}

	raw, err := os.ReadFile(fn)

	if err != nil {
		return err
	}

	off, sz1 := 0x004, 16

	rlen := len(raw)

	if rlen < off+128*sz1 {
		return os.ErrInvalid
	}

	setup := uint16(mid.BnkIBK)
	if bi.IsMT32 {
		setup = mid.BnkIBKMT32
	}

	bid := banks.AddBank(id, bi.Name, setup)
	mb := mid.NewMidiBank(0, 0)

	for i := range 128 {
		ii := off + i*sz1

		var (
			def idef
			ops [4]mid.OP
		)

		def[0] = raw[ii+0]
		def[1] = raw[ii+1]
		def[8] = raw[ii+2]
		def[9] = raw[ii+3]
		def[2] = raw[ii+4]
		def[3] = raw[ii+5]
		def[4] = raw[ii+6]
		def[5] = raw[ii+7]
		def[6] = raw[ii+8]
		def[7] = raw[ii+9]
		def[10] = raw[ii+10]

		toOPs(def, ops[:], 0)

		ins := mid.NewInsEntry()
		ins.SetFBConn(uint16(raw[ii+10]), 0)

		ins.Note.Off0 = int8(raw[ii+12])

		if perc {
			ins.Note.Off0 = 0
			ins.DrumKey = raw[ii+13]

			if !bi.IsMT32 {
				switch raw[ii+11] {
				case 6:
					ins.Flags |= mid.DrumBass
				case 7:
					ins.Flags |= mid.DrumSnare
				case 8:
					ins.Flags |= mid.DrumTomTom
				case 9:
					ins.Flags |= mid.DrumCymbal
				case 10:
					ins.Flags |= mid.DrumHiHat
				default:
					ins.Flags |= mid.FlagSilence
				}
			}
		}

		banks.AddIns(mb, i, ins, ops[:], fn)
	}

	banks.AddMidi(bid, mb, perc)

	return nil
}
