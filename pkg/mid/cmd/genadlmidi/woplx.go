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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func LoadWOPLX(banks *mid.BankDB, id int, bi BankInfo) error {
	fn := bi.FileM

	oplx, err := parseBank(fn)
	if err != nil {
		return err
	}

	var setup uint16
	for k, v := range oplx.hdr {
		n, _ := strconv.ParseUint(v, 10, 16)
		switch k {
		case "DEEP_VIBRATO":
			setup |= uint16(n&1) << 9
		case "DEEP_TREMOLO":
			setup |= uint16(n&1) << 8
		case "VOLUME_MODEL":
			setup |= uint16(n) & 0xFF
		case "IS_MT32":
			setup |= uint16(n&1) << 10
		}
	}

	bid := banks.AddBank(id, bi.Name, setup)

	mlsb := uint8(oplx.ids[0].lsb)
	mmsb := uint8(oplx.ids[0].msb)

	melo := mid.NewMidiBank(mlsb, mmsb)

	dlsb := uint8(oplx.ids[1].lsb)
	dmsb := uint8(oplx.ids[1].msb)

	drum := mid.NewMidiBank(dlsb, dmsb)

	load := func(mb *mid.MidiBank, xb []xins, percussion bool) {
		for _, mi := range xb {
			ie := mi.insEntry(percussion)

			ops := mi.Ops()

			banks.AddIns(mb, mi.id, ie, ops[:], fn)
		}
	}

	load(melo, oplx.melos, false)
	load(drum, oplx.drums, true)

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}

type xbank struct {
	hdr map[string]string
	ids [2]struct {
		lsb, msb int
	}
	melos []xins
	drums []xins
}

type xins struct {
	id     int
	name   string
	flags  string
	attrs  map[string]int
	fbconn map[string]int
	ops    [4]xop
}

func (x xins) FBConn() uint16 {
	var fb16 uint16

	for k, v := range x.fbconn {
		switch k {
		case "FB1":
			fb16 |= (uint16(v) << 1) & 0x0E
		case "FB2":
			fb16 |= ((uint16(v) << 1) & 0x0E) << 8
		case "CONN1":
			fb16 |= uint16(v) & 1
		case "CONN2":
			fb16 |= (uint16(v) & 1) << 8
		}
	}

	return fb16
}

func (x xins) Flags() int {
	var flags int

	fields := strings.FieldsFunc(x.flags, func(x rune) bool { return x == ';' })

	for _, f := range fields {
		switch f {
		case "FN":
			flags |= mid.FlagFixedTone
		case "2OP":
			flags |= mid.Flag2op
		case "DV":
			flags |= mid.Flag4op | mid.FlagPseudo4op
		case "4OP":
			flags |= mid.Flag4op
		}
	}

	return flags
}

func (x xins) insEntry(percussion bool) *mid.InsEntry {
	ie := mid.NewInsEntry()
	ie.FBConn = x.FBConn()
	ie.Flags = x.Flags()
	x.Attrs(ie)

	if _, hasDrumKey := x.attrs["DRUM_KEY"]; percussion && hasDrumKey {
		ie.Flags |= mid.FlagFixedTone
	}

	return ie
}

func (x xins) Ops() [4]mid.OP {
	var ops [4]mid.OP

	defs := x.Defs()
	toOPs(defs[0], ops[:], 0)
	toOPs(defs[1], ops[:], 2)

	return ops
}

func (x xins) Defs() [2]idef {
	var defs [2]idef

	nop := max(len(x.ops), 4)

	for i := range nop {
		op := x.ops[i]

		def := &defs[i/2]
		p := i & 1

		def[0+p] = byte((op[xam]&1)<<7 | (op[xvb]&1)<<6 | (op[xeg]&1)<<5 | (op[xkr]&1)<<4 | op[xml]&0x0F)
		def[2+p] = byte((op[xat]&0x0F)<<4 | op[xdc]&0x0F)
		def[4+p] = byte((op[xst]&0x0F)<<4 | op[xrl]&0x0F)
		def[6+p] = byte(op[xwf] & 0x07)
		def[8+p] = byte((op[xkl]&0x03)<<6 | op[xtl]&0x3F)
	}

	fbconn := x.FBConn()
	defs[0][10] = lo8(fbconn)
	defs[1][10] = hi8(fbconn)

	return defs
}

func (x xins) Attrs(ie *mid.InsEntry) {
	ie.Note.Off0 = int8(x.attrs["NOTE_OFF0"])
	ie.Note.Off1 = int8(x.attrs["NOTE_OFF1"])
	ie.VelOff = int8(x.attrs["VEL_OFF"])
	ie.SecVoice = float64(x.attrs["SEC_VOICE"])
	ie.DrumKey = uint8(x.attrs["DRUM_KEY"])
	ie.KON = time.Duration(x.attrs["KON"]) * time.Millisecond
	ie.KOFF = time.Duration(x.attrs["KOFF"]) * time.Millisecond
	ie.HasStoredTimings = true
}

func parseBank(path string) (*xbank, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	bank := new(xbank{
		hdr: map[string]string{},
	})

	var (
		section string // "", "BANK_INFO", "MELODIC_BANK", "PERCUSSION_BANK"
		id      struct {
			lsb, msb int
		}
		current *xins // instrument being filled
	)

	flush := func() {
		if current == nil {
			return
		}

		switch section {
		case "MELODIC_BANK":
			bank.ids[0] = id
			bank.melos = append(bank.melos, *current)
		case "PERCUSSION_BANK":
			bank.ids[1] = id
			bank.drums = append(bank.drums, *current)
		}

		current = nil
	}

	scanner := bufio.NewScanner(f)
	// Some lines (BANK_INFO text) can be long; bump the buffer.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	ok := scanner.Scan()
	if !ok {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("empty file")
	}

	if !strings.HasPrefix(strings.TrimSpace(scanner.Text()), "WOPLX-BANK") {
		return nil, fmt.Errorf("missing WOPLX-BANK header")
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip blanks and pure comment-ish lines we don't care about.
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}

		switch line {
		case "BANK_INFO:", "MELODIC_BANK:", "PERCUSSION_BANK:":
			flush()
			section = strings.TrimSuffix(line, ":")
			continue
		case "BANK_INFO_END", "MELODIC_BANK_END", "PERCUSSION_BANK_END":
			flush()
			section = ""
			continue
		}

		// Inside the free-text BANK_INFO block: ignore everything.
		if section == "BANK_INFO" {
			continue
		}

		// INSTRUMENT=n:
		if strings.HasPrefix(line, "INSTRUMENT=") {
			flush()

			s := strings.TrimSuffix(strings.TrimPrefix(line, "INSTRUMENT="), ":")

			id, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("bad instrument index %q: %w", s, err)
			}

			current = &xins{
				id:     id,
				attrs:  map[string]int{},
				fbconn: map[string]int{},
			}

			continue
		}

		if current == nil {
			switch section {
			case "":
				if k, v, ok := strings.Cut(line, "="); ok {
					bank.hdr[strings.TrimSpace(k)] = strings.TrimSpace(v)
				}
			case "MELODIC_BANK", "PERCUSSION_BANK":
				if k, v, ok := strings.Cut(line, "="); ok {
					switch k {
					case "MIDI_BANK_LSB":
						id.lsb, _ = strconv.Atoi(strings.TrimSpace(v))
					case "MIDI_BANK_MSB":
						id.msb, _ = strconv.Atoi(strings.TrimSpace(v))
					}
				}
			}

			continue
		}

		// NAME=...
		if k, v, ok := strings.Cut(line, "="); ok && k == "NAME" {
			current.name = strings.TrimSpace(v)
			continue
		}

		// FLAGS: 2OP;
		if tail, ok := strings.CutPrefix(line, "FLAGS:"); ok {
			current.flags = strings.Trim(strings.TrimSpace(tail), ";")
			continue
		}

		// ATTRS: K=V;K=V;
		if tail, ok := strings.CutPrefix(line, "ATTRS:"); ok {
			parseKV(current.attrs, tail)
			continue
		}

		// FBCONN: FB1=3;CONN1=0;
		if tail, ok := strings.CutPrefix(line, "FBCONN:"); ok {
			parseKV(current.fbconn, tail)
			continue
		}

		for i, p := range []string{"OP0:", "OP1:", "OP2:", "OP3:"} {
			// OPx: AT=13;DC=2;...
			if tail, ok := strings.CutPrefix(line, p); ok {
				m := make(map[string]int, 12)
				parseKV(m, tail)

				current.ops[i] = xop{
					xat: m["AT"], xdc: m["DC"], xst: m["ST"], xrl: m["RL"],
					xwf: m["WF"], xml: m["ML"], xtl: m["TL"], xkl: m["KL"],
					xvb: m["VB"], xam: m["AM"], xeg: m["EG"], xkr: m["KR"],
				}

				break
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return bank, nil
}

const (
	xat = iota
	xdc
	xst
	xrl
	xwf
	xml
	xtl
	xkl
	xvb
	xam
	xeg
	xkr
)

type xop [12]int

// parseKV fills dst from a string like "FB1=3;CONN1=0;".
func parseKV(dst map[string]int, s string) {
	for part := range strings.SplitSeq(s, ";") {

		part = strings.TrimSpace(part)

		if part == "" {
			continue
		}

		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}

		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			continue
		}

		dst[strings.TrimSpace(k)] = n
	}
}
