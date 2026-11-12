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
	"os"
	"path/filepath"
	"testing"
	"time"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func TestLoadWOPLTimingPresenceAndDeduplication(t *testing.T) {
	dir := t.TempDir()
	db := mid.NewBankDB()
	for version := uint16(1); version <= 3; version++ {
		bank := NewWOPLFile(version, 0, 0, 1, 1)
		for i := range 128 {
			bank.melos[0].ins[i].flags = WOPLInsBlank
			bank.drums[0].ins[i].flags = WOPLInsBlank
		}
		bank.melos[0].ins[0].flags = 0
		bank.melos[0].ins[1].flags = 0
		bank.melos[0].ins[1].kon = 123 * time.Millisecond
		bank.melos[0].ins[1].koff = 456 * time.Millisecond
		data := make([]byte, CalculateBankFileSize(bank, version))
		if err := SaveBankToMem(bank, data, version, false); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, fmt.Sprintf("v%d.wopl", version))
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := LoadWOPL(db, int(version), BankInfo{Name: fmt.Sprintf("v%d", version), FileM: path}); err != nil {
			t.Fatal(err)
		}
	}

	instrument := func(bank, patch int) *mid.InsEntry {
		t.Helper()
		index := db.Midis[db.Banks[bank].Melos[0]].Ins[patch]
		if index < 0 || index >= len(db.Ins) {
			t.Fatalf("bank %d patch %d has invalid instrument %d", bank, patch, index)
		}
		return db.Ins[index]
	}
	v1, v2, v3zero, v3timed := instrument(0, 0), instrument(1, 0), instrument(2, 0), instrument(2, 1)
	if v1.HasStoredTimings || v2.HasStoredTimings || v1 != v2 {
		t.Fatal("v1/v2 instruments should have missing timings and deduplicate")
	}
	if !v3zero.HasStoredTimings || v3zero.KON != 0 || v3zero.KOFF != 0 || v3zero == v1 {
		t.Fatal("v3 zero timings must be distinguished from absent timings")
	}
	if !v3timed.HasStoredTimings || v3timed.KON != 123*time.Millisecond || v3timed.KOFF != 456*time.Millisecond || v3timed == v3zero {
		t.Fatal("v3 nonzero timings were not preserved or deduplicated correctly")
	}

	// Older versions have no durations, so both patches are identical.
	if instrument(0, 1) != v1 || instrument(1, 1) != v2 {
		t.Fatal("v1/v2 timing bytes were incorrectly treated as present")
	}
	loadedPath := filepath.Join(dir, "database.gob")
	if err := db.Dump(loadedPath); err != nil {
		t.Fatal(err)
	}
	loaded, err := mid.OpenDB(loadedPath)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loaded.AsTables()
	if err != nil {
		t.Fatal(err)
	}
	meta := tables.Banks[2].Melos[0].Programs[1]
	if got := tables.Metas[meta]; got.KON != 123*time.Millisecond || got.KOFF != 456*time.Millisecond {
		t.Errorf("round-tripped playback timings = %s/%s", got.KON, got.KOFF)
	}
}
