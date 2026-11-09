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
	"encoding/json/v2"
	"fmt"
	"iter"
	"maps"
	"os"
	"slices"
	"strings"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

type Config struct {
	General `json:"general"`
	Banks   map[string]BankInfo `json:"banks"`

	db *mid.BankDB
}

type General struct {
	BankCount uint `json:"banks"`
	AutoID    bool `json:"auto_id"`
}

type BankInfo struct {
	Name    string   `json:"name"`
	Format  string   `json:"format"`
	FileM   string   `json:"file"`
	FileD   string   `json:"file-p"`
	PrefixM string   `json:"prefix"`
	PrefixD string   `json:"prefix-p"`
	FilterM string   `json:"filter-m"`
	FilterD string   `json:"filter-p"`
	IsMT32  FlexBool `json:"mt32-defaults"`
}

func (cfg *Config) Open(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = json.Unmarshal(raw, cfg)
	if err != nil {
		return err
	}

	nbank := cfg.BankCount

	if nbank == 0 {
		return fmt.Errorf("no bank found")
	}

	var nproc uint

	for i, bnk := range allSortedInfos(cfg.Banks) {
		err := cfg.processBank(i, bnk)

		if err != nil {
			return err
		}

		nproc++
	}

	if nproc != nbank {
		return fmt.Errorf("expected %d banks, but processed %d", nbank, nproc)
	}

	return nil
}

func (cfg *Config) DB() *mid.BankDB {
	return cfg.db
}

type loader func(BankStore *mid.BankDB, id int, b BankInfo) error

var loaders = map[string]loader{
	"AdLibGold":    LoadBNK2,
	"AIL":          LoadMiles,
	"Bisqwit":      LoadBisqwit,
	"EA":           LoadEA,
	"HMI":          LoadBNK,
	"IBK":          LoadIBK,
	"Junglevision": LoadJunglevision,
	"OP2":          LoadDoom,
	"TMB":          LoadTMB,
	"WOPL":         LoadWOPL,
	"WOPLX":        LoadWOPLX,
}

func (cfg *Config) processBank(id int, bnk BankInfo) error {
	load, ok := loaders[bnk.Format]

	if !ok {
		return fmt.Errorf("unsupported bank format: %s", bnk.Format)
	}

	if cfg.db == nil {
		cfg.db = new(mid.BankDB)
	}

	return load(cfg.db, id, bnk)
}

func allSortedInfos(banks map[string]BankInfo) iter.Seq2[int, BankInfo] {
	return func(yield func(int, BankInfo) bool) {
		keys := slices.Sorted(maps.Keys(banks))

		for i, k := range keys {
			if !yield(i, banks[k]) {
				return
			}
		}

	}
}

type FlexBool bool

func (b *FlexBool) UnmarshalJSON(data []byte) error {
	// Try plain JSON bool first
	var v bool
	if err := json.Unmarshal(data, &v); err == nil {
		*b = FlexBool(v)
		return nil
	}
	// Fall back to string: "true" / "false" / "1" / "0"
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("FlexBool: expected bool or string, got %s", data)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "1", "yes", "on":
		*b = true
	case "false", "0", "no", "off", "":
		*b = false
	default:
		return fmt.Errorf("FlexBool: cannot interpret %q as bool", s)
	}
	return nil
}

func (b FlexBool) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(b)) // always emits a real bool on write
}
