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
	"path/filepath"
	"testing"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestBuiltinGENMIDITimingsMatchWOPL(t *testing.T) {
	path := filepath.Join("fm_banks_new", "wopl_files", "GENMIDI.wopl")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("optional GENMIDI.wopl fixture not installed")
	}
	if err != nil {
		t.Fatal(err)
	}
	source, err := LoadBankFromMem(data)
	if err != nil {
		t.Fatal(err)
	}
	if source.ver != 3 {
		t.Fatalf("GENMIDI version = %d, want 3", source.ver)
	}
	db, err := mid.OpenDB(filepath.Join("..", "..", "adlmidi.dat"))
	if err != nil {
		t.Fatal(err)
	}
	var selected *mid.BankEntry
	for _, bank := range db.Banks {
		if bank.Name == "WOPL (DMXOPL3 GM)" {
			selected = bank
			break
		}
	}
	if selected == nil {
		t.Fatal("GENMIDI bank not found")
	}
	count := 0
	for _, group := range []struct {
		source []WOPLBank
		ids    []int
	}{{source.melos, selected.Melos}, {source.drums, selected.Drums}} {
		if len(group.source) != len(group.ids) {
			t.Fatalf("bank counts differ: source %d, embedded %d", len(group.source), len(group.ids))
		}
		for i, bank := range group.source {
			midiBank := db.Midis[group.ids[i]]
			for patch, raw := range bank.ins {
				if raw.flags&WOPLInsBlank != 0 {
					continue
				}
				idx := midiBank.Ins[patch]
				if idx < 0 || idx >= len(db.Ins) {
					t.Fatalf("bank %d patch %d lost nonblank instrument", i, patch)
				}
				ins := db.Ins[idx]
				if !ins.HasStoredTimings || ins.KON != raw.kon || ins.KOFF != raw.koff || ins.Flags&mid.FlagSilence != 0 {
					t.Fatalf("bank %d patch %d: stored %t, %s/%s flags %#x; source %s/%s", i, patch, ins.HasStoredTimings, ins.KON, ins.KOFF, ins.Flags, raw.kon, raw.koff)
				}
				count++
			}
		}
	}
	if count == 0 {
		t.Fatal("no nonblank GENMIDI instruments found")
	}
}
