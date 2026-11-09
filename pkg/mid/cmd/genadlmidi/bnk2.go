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
	"bytes"
	"os"
	"strconv"
	"strings"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func LoadBNK2(banks *mid.BankDB, id int, bi BankInfo) error {
	fmelo, fperc := bi.FilterM, bi.FilterD

	raw, err := os.ReadFile(bi.FileM)

	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(raw[:40]), "Accomp. Bank, (C) AdLib Inc") {
		return os.ErrInvalid
	}

	bid := banks.AddBank(id, bi.Name, mid.BnkHMI)
	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	nentry := u16(raw[40:])
	data := raw[48:]

	for i := range nentry {
		off0 := i * 28

		used := int(data[off0+15])

		if used == 0 {
			continue
		}

		nam0 := data[off0+3 : off0+15]
		if i := bytes.IndexByte(nam0, 0); i >= 0 {
			nam0 = nam0[:i]
		}

		var perc bool

		gmid := 0
		switch {
		case strings.HasPrefix(string(nam0), fmelo):
			s := string(nam0[len(fmelo):])

			n, ok := toInt(s)

			if !ok {
				continue
			}

			gmid = n
		case strings.HasPrefix(string(nam0), fperc):
			s := string(nam0[len(fperc):])

			n, ok := toInt(s)

			if !ok {
				continue
			}

			gmid = n + 0x80
			perc = true
		default:
			continue
		}

		// attrib := i32(data[off0+16:])
		off1 := i32(data[off0+20:])

		in := raw[off1:]
		rops := [4][]byte{
			in[0:6],
			in[6:12],
			in[12:18],
			in[18:24],
		}

		C4xxxFFFC := in[24]
		xxP24NNN := in[25]
		TTTTTTTT := int8(in[26])

		var (
			defs [2]idef
			ops  [4]mid.OP
		)

		for j := range defs {
			defs[j][0] = rops[2*j+0][0]
			defs[j][1] = rops[2*j+1][0]
			defs[j][2] = rops[2*j+0][2]
			defs[j][3] = rops[2*j+1][2]
			defs[j][4] = rops[2*j+0][3]
			defs[j][5] = rops[2*j+1][3]
			defs[j][6] = rops[2*j+0][4] & 0x07
			defs[j][7] = rops[2*j+1][4] & 0x07
			defs[j][8] = rops[2*j+0][1]
			defs[j][9] = rops[2*j+1][1]

			toOPs(defs[j], ops[:], 2*j)
		}
		defs[0][10] = C4xxxFFFC & 0x0F
		defs[1][10] = C4xxxFFFC&0x0E | C4xxxFFFC>>7

		ins := mid.NewInsEntry()
		ins.Note.Off1 = TTTTTTTT
		ins.SetFBConn(uint16(defs[0][10]), uint16(defs[1][10]))

		if xxP24NNN&8 != 0 {
			ins.Flags |= mid.Flag4op
		}

		mb := melo
		if perc {
			mb = drum
			ins.DrumKey = 35
		}

		banks.AddIns(mb, gmid, ins, ops[:], bi.FileM)
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}

func toInt(s string) (int, bool) {
	i := 0

	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}

	if i == 0 {
		return 0, false
	}

	n, err := strconv.Atoi(s[:i])
	return n, err == nil
}
