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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertAddsWOPLBanksWithNextIDs(t *testing.T) {
	dir := t.TempDir()
	iniPath := filepath.Join(dir, "banks.ini")
	jsonPath := filepath.Join(dir, "banks.json")
	ini := "[General]\nbanks=2\nauto_id=true\n[bank-00]\nname=First\n[bank-01]\nname=Second\n"
	if err := os.WriteFile(iniPath, []byte(ini), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := convert(iniPath, jsonPath); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		General General                      `json:"general"`
		Banks   map[string]map[string]string `json:"banks"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.General.BankCount != 5 || !got.General.AutoID {
		t.Errorf("general = %+v, want 5 banks with auto_id", got.General)
	}
	want := map[string]string{
		"bank-02": "fm_banks_new/wopl_files/GENMIDI.wopl",
		"bank-03": "fm_banks_new/wopl_files/GENMIDI(XG).wopl",
		"bank-04": "fm_banks_new/wopl_files/GENMIDI(GS).wopl",
	}
	for key, file := range want {
		if got.Banks[key]["file"] != file {
			t.Errorf("%s file = %q, want %q", key, got.Banks[key]["file"], file)
		}
	}
}
