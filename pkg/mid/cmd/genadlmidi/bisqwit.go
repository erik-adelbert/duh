// duh ADL midi driver for OPL3
//
// Based on:
//
//	Wohlstand libADLMIDI
//	bisqwit adlmidi
//
// Uses:
//
//	sneakernets DMXOPL wopl definitions
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
	"bufio"
	"os"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func LoadBisqwit(banks *mid.BankDB, id int, bi BankInfo) error {
	f, err := os.Open(bi.FileM)

	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck // can't do anything if it fails
	bf := bufio.NewReader(f)

	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	bid := banks.AddBank(id, bi.Name, mid.BnkGeneric)

	patch := 0
	for i := range 256 {
		var (
			dbuf [2]idef
			ops  [4]mid.OP
		)

		note, _ := bf.ReadByte()

		for i := range dbuf {
			_, err = bf.Read(dbuf[i][:])

			if err != nil {
				return err
			}
		}

		ins := mid.NewInsEntry()

		toOPs(dbuf[0], ops[:], 0)
		if dbuf[0] != dbuf[1] {
			ins.Flags |= mid.Flag4op
			toOPs(dbuf[1], ops[:], 2)
		}

		ins.FBConn = uint16(dbuf[0].fb(0)) | (uint16(dbuf[1].fb(1)) << 8)
		ins.Note.Off0 = int8(note)

		mb := melo
		if i >= 128 {
			mb = drum

			ins.DrumKey = uint8(note)
			ins.Note.Off0 = 0
		}

		banks.AddIns(mb, patch, ins, ops[:], bi.FileM)

		if patch++; patch >= 128 {
			patch = 0
		}
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}
