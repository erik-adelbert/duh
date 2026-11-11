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
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func convert(ini, conf string) error {
	in, err := os.Open(ini)
	if err != nil {
		return err
	}
	defer in.Close() //nolint:errcheck // can't do anything if it fails

	var (
		section string
		current map[string]string
	)

	sections := make(map[string]map[string]string)

	scanner := bufio.NewScanner(in)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		switch {
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			section = line[1 : len(line)-1]

			var bid int

			if n, err := fmt.Sscanf(section, "bank-%d", &bid); n == 1 && err == nil {
				section = fmt.Sprintf("bank-%02d", bid)
			}

			if _, ok := sections[section]; !ok {
				sections[section] = make(map[string]string)
			}
			current = sections[section]
		case line == "" || strings.HasPrefix(line, ";"):
			// skip empty lines and comments
		default:
			// handle key=value lines
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				current[key] = strings.Trim(value, "\"")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	x, ok := sections["General"]["auto_id"]
	autoid := ok && x == "true"

	delete(sections, "General")
	dmxBanks := []map[string]string{
		{
			"file":   "fm_banks_new/wopl_files/GENMIDI.wopl",
			"format": "WOPL",
			"games":  "GENMIDI",
			"name":   "WOPL (DMXOPL3 GM)",
			"prefix": "gm",
		},
		{
			"file":   "fm_banks_new/wopl_files/GENMIDI(XG).wopl",
			"format": "WOPL",
			"games":  "Yamaha XG",
			"name":   "WOPL (DMXOPL3 XG)",
			"prefix": "xg",
		},
		{
			"file":   "fm_banks_new/wopl_files/GENMIDI(GS).wopl",
			"format": "WOPL",
			"games":  "Roland GS",
			"name":   "WOPL (DMXOPL3 GS)",
			"prefix": "gs",
		},
	}
	for _, bank := range dmxBanks {
		present := false
		for _, section := range sections {
			if section["file"] == bank["file"] {
				present = true
				break
			}
		}
		if present {
			continue
		}

		id := len(sections)
		for {
			key := fmt.Sprintf("bank-%02d", id)
			if _, exists := sections[key]; !exists {
				sections[key] = bank
				break
			}
			id++
		}
	}

	var cf struct {
		General General                      `json:"general"`
		Banks   map[string]map[string]string `json:"banks"`
	}
	cf.General = General{
		BankCount: uint(len(sections)),
		AutoID:    autoid,
	}

	cf.Banks = sections

	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(conf, data, 0644)
}
