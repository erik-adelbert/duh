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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestWOPLVersionsAndEndianFields(t *testing.T) {
	for _, version := range []uint16{1, 2, 3} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			instrument := &WOPLIFile{drum: true}
			instrument.ins.note.off0 = -258
			instrument.ins.secvod = -64
			data := make([]byte, CalculateInsFileSize(instrument, version))
			if err := SaveInsToMem(instrument, data, version); err != nil {
				t.Fatal(err)
			}
			if data[11] != byte(version) || data[12] != 0 || data[14+32] != 0xfe || data[14+33] != 0xfe {
				t.Fatalf("unexpected instrument version or note offset bytes: %x %x", data[11:13], data[46:48])
			}
			gotIns, err := LoadInsFromMem(data)
			if err != nil {
				t.Fatal(err)
			}
			if gotIns.ver != version || !gotIns.drum || gotIns.ins.note.off0 != -258 || gotIns.ins.secvod != -64 {
				t.Fatalf("unexpected instrument: %#v", gotIns)
			}

			bank := NewWOPLFile(version, 0, 0, 2, 1)
			bank.melos[1].ins[0].note.off1 = 0x1234
			bank.melos[1].ins[0].kon = 0x1234 * time.Millisecond
			data = make([]byte, CalculateBankFileSize(bank, version))
			if err := SaveBankToMem(bank, data, version, false); err != nil {
				t.Fatal(err)
			}
			if data[11] != byte(version) || data[12] != 0 || data[13] != 0 || data[14] != 2 || data[15] != 0 || data[16] != 1 {
				t.Fatalf("unexpected bank version or count bytes: %x", data[11:17])
			}
			gotBank, err := LoadBankFromMem(data)
			if err != nil {
				t.Fatal(err)
			}
			if gotBank.ver != version || len(gotBank.melos) != 2 || len(gotBank.drums) != 1 || gotBank.melos[1].ins[0].note.off1 != 0x1234 {
				t.Fatalf("unexpected bank version, counts, or note offset")
			}
			if version == 3 && gotBank.melos[1].ins[0].kon != 0x1234*time.Millisecond {
				t.Errorf("key-on duration = %s", gotBank.melos[1].ins[0].kon)
			}
		})
	}
}

func TestWOPLVersionErrors(t *testing.T) {
	for _, magic := range []string{WOPLINS, WOPLMAGIC} {
		for _, tc := range []struct {
			version uint16
			want    error
		}{{0, errInvalidVersion}, {WOPLVER + 1, errNewerVersion}} {
			t.Run(fmt.Sprintf("%s/%d", magic[:10], tc.version), func(t *testing.T) {
				data := make([]byte, 13)
				copy(data, magic)
				data[11], data[12] = byte(tc.version), byte(tc.version>>8)
				var err error
				if magic == WOPLINS {
					_, err = LoadInsFromMem(data)
				} else {
					_, err = LoadBankFromMem(data)
				}
				if !errors.Is(err, tc.want) {
					t.Fatalf("error = %v, want %v", err, tc.want)
				}
			})
		}
	}
}

func TestWOPLDetuneRoundTrip(t *testing.T) {
	for value := -128; value <= 127; value++ {
		instrument := &WOPLIFile{}
		instrument.ins.secvod = int8(value)
		data := make([]byte, CalculateInsFileSize(instrument, 3))
		if err := SaveInsToMem(instrument, data, 3); err != nil {
			t.Fatal(err)
		}
		if data[14+37] != byte(int8(value)) {
			t.Fatalf("detune %d encoded as %x", value, data[14+37])
		}
		got, err := LoadInsFromMem(data)
		if err != nil {
			t.Fatal(err)
		}
		if got.ins.secvod != int8(value) {
			t.Fatalf("detune %d decoded as %d", value, got.ins.secvod)
		}
	}
}

func TestLoadWOPLDetuneScaling(t *testing.T) {
	cases := []struct {
		raw  int8
		want float64
	}{
		{-128, -2}, {-65, -1.03125}, {-1, -0.03125},
		{0, 0}, {1, 0}, {64, 1}, {127, 1.96875},
	}
	bank := NewWOPLFile(3, 0, 0, 1, 1)
	for i := range bank.melos[0].ins {
		bank.melos[0].ins[i].flags = WOPLInsBlank
		bank.drums[0].ins[i].flags = WOPLInsBlank
	}
	for i, tc := range cases {
		bank.melos[0].ins[i].flags = WOPLIns4Op | WOPLInsPseudo4Op
		bank.melos[0].ins[i].secvod = tc.raw
		for op := range 4 {
			bank.melos[0].ins[i].ops[op].avek20 = uint8(0x10 + op)
		}
	}
	data := make([]byte, CalculateBankFileSize(bank, 3))
	if err := SaveBankToMem(bank, data, 3, false); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "detune.wopl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	db := mid.NewBankDB()
	if err := LoadWOPL(db, 0, BankInfo{Name: "detune", FileM: path}); err != nil {
		t.Fatal(err)
	}
	tables, err := db.AsTables()
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range cases {
		midiBank := db.Midis[db.Banks[0].Melos[0]]
		got := db.Ins[midiBank.Ins[i]].SecVoice
		meta := tables.Banks[0].Melos[0].Programs[i]
		if got != tc.want || tables.Metas[meta].SecondFinetune != tc.want {
			t.Errorf("raw detune %d: database %g, playback %g; want %g", tc.raw, got, tables.Metas[meta].SecondFinetune, tc.want)
		}
		if tables.Metas[meta].ID0 == tables.Metas[meta].ID1 || !mid.IsPseudo4Op(tables.Metas[meta].Flags) {
			t.Errorf("raw detune %d: pseudo-4op lost its second voice: %+v", tc.raw, tables.Metas[meta])
		}
		first, second := tables.Instruments[tables.Metas[meta].ID0], tables.Instruments[tables.Metas[meta].ID1]
		if uint8(first.ModulatorE862) != 0x11 || uint8(first.CarrierE862) != 0x10 ||
			uint8(second.ModulatorE862) != 0x13 || uint8(second.CarrierE862) != 0x12 {
			t.Errorf("incorrect WOPL operator order: first %+v, second %+v", first, second)
		}
	}
}

func TestLoadWOPLBankSettingsAndRhythm(t *testing.T) {
	bank := NewWOPLFile(3, WOPLFlagDeepTremolo|WOPLFileDeepVibrato|WOPLFileMT32, mid.VolAIL, 1, 1)
	bank.drums[0].ins[42].flags = mid.FlagRMSnare
	bank.drums[0].ins[42].drumid = 36
	data := make([]byte, CalculateBankFileSize(bank, 3))
	if err := SaveBankToMem(bank, data, 3, false); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "settings.wopl")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	db := mid.NewBankDB()
	if err := LoadWOPL(db, 0, BankInfo{Name: "settings", FileM: path}); err != nil {
		t.Fatal(err)
	}
	tables, err := db.AsTables()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := tables.Banks[0].Setup, uint16(mid.VolAIL|7<<8); got != want {
		t.Errorf("bank setup = %#x, want %#x", got, want)
	}
	meta := tables.Banks[0].Drums[0].Programs[42]
	if got := tables.Metas[meta].Flags & mid.FlagRhythmMask; got != mid.FlagRMSnare {
		t.Errorf("rhythm flags = %#x, want snare", got)
	}
	if got := tables.Metas[meta].Tone; got != 36 {
		t.Errorf("drum tone = %d, want 36", got)
	}
}

func TestLoadRealWOPLPseudoFourOp(t *testing.T) {
	path := filepath.Join("fm_banks_new", "wopl_files", "GENMIDI.wopl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("optional GENMIDI.wopl fixture not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	raw, err := LoadBankFromMem(data)
	if err != nil {
		t.Fatal(err)
	}
	db := mid.NewBankDB()
	if err := LoadWOPL(db, 0, BankInfo{Name: "GENMIDI", FileM: path}); err != nil {
		t.Fatal(err)
	}
	tables, err := db.AsTables()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for patch, ins := range raw.melos[0].ins {
		if ins.flags&WOPLInsBlank != 0 || ins.flags&(WOPLIns4Op|WOPLInsPseudo4Op) != WOPLIns4Op|WOPLInsPseudo4Op {
			continue
		}
		metaID := tables.Banks[0].Melos[0].Programs[patch]
		meta := tables.Metas[metaID]
		if !mid.IsPseudo4Op(meta.Flags) || meta.ID0 == meta.ID1 {
			t.Fatalf("GENMIDI patch %d lost its second voice: %+v", patch, meta)
		}
		second := tables.Instruments[meta.ID1]
		if uint8(second.ModulatorE862) != ins.ops[WOPLOPMod1].avek20 ||
			uint8(second.CarrierE862) != ins.ops[WOPLOPCar1].avek20 {
			t.Fatalf("GENMIDI patch %d has incorrect second-voice operators", patch)
		}
		count++
	}
	if count == 0 {
		t.Fatal("GENMIDI fixture contains no melodic pseudo-4op patches")
	}
}
