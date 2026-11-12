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
	"fmt"
	"io"
	"os"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

type GTLHeader struct {
	Patch, Bank uint8
	Offset      uint32
}

func LoadMiles(banks *mid.BankDB, id int, bi BankInfo) error {
	raw, err := os.ReadFile(bi.FileM)

	if err != nil {
		return err
	}

	maxid := 0
	gtls := make([]GTLHeader, 0, 256)

GTLScan:
	for i := range cap(gtls) {
		off := i * 6

		switch {
		case raw[off] == 0xFF, raw[off+1] == 0xFF:
			break GTLScan
		case raw[off] > 0x7F:
			return fmt.Errorf("invalid patch value: %x", raw[off])
		}

		maxid = max(maxid, int(raw[off+1]))

		gtls = append(gtls, GTLHeader{
			Patch:  raw[off],
			Bank:   raw[off+1],
			Offset: u32(raw[off+2 : off+6]),
		})
	}

	setup := uint16(mid.BnkAIL)
	if bi.IsMT32 {
		setup = mid.BnkAILMT32
	}

	bid := banks.AddBank(id, bi.Name, setup)

	drum := mid.NewMidiBank(0, 0)

	melos := make([]*mid.MidiBank, maxid+1)

	var lsb8, msb8 uint8

	for i := range melos {
		lsb8 = uint8(i)

		if lsb8 == 0 {
			msb8++
		}

		melos[i] = mid.NewMidiBank(lsb8, msb8)
	}

	for i := range gtls {
		h := gtls[i]

		perc := (h.Bank == 0x7F)

		gmid := int(h.Patch)
		if perc {
			gmid |= 0x80
		}

		off0 := h.Offset

		note := int8(raw[off0+2])
		nins := (u16(raw[off0:]) - 3) / 11

		var (
			defs [2]idef
			ops  [4]mid.OP
		)

		ins := mid.NewInsEntry()
		for j := range min(nins, 2) {
			lo := off0 + 3 + uint32(11*j)
			hi := lo + 11

			if hi > uint32(len(raw)) {
				return io.ErrUnexpectedEOF
			}

			raw := raw[lo:hi]

			defs[j][0] = raw[0]
			defs[j][8] = raw[1]
			defs[j][2] = raw[2]
			defs[j][4] = raw[3]
			defs[j][6] = raw[4]
			defs[j][1] = raw[6]
			defs[j][9] = raw[7]
			defs[j][3] = raw[8]
			defs[j][5] = raw[9]
			defs[j][7] = raw[10]

			fbc := raw[8]
			defs[j][10] = fbc

			if j == 1 {
				ins.Flags |= mid.Flag4op
				defs[0][10] = fbc & 0x0F
				defs[1][10] = (fbc & 0xE0) | (fbc >> 7)
			}

			toOPs(defs[j], ops[:], 2*int(j))

			if nins <= 2 {
				ins.Note.Off0 = int8(note)

				ins.FBConn = uint16(fbc&0x0F) | ((uint16(fbc&0xE0) | (uint16(fbc) >> 7)) << 8)

				mb := melos[h.Bank]
				if perc {
					mb = drum

					ins.DrumKey = uint8(note)
					ins.Note.Off0 = 0
				}

				banks.AddIns(mb, gmid, ins, ops[:], bi.FileM)
			}
		}
	}

	for i := range melos {
		banks.AddMidi(bid, melos[i], false)
	}
	banks.AddMidi(bid, drum, true)

	return nil
}
