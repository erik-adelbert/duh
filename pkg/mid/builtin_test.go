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

package mid

import (
	"strings"
	"testing"
)

func TestDefaultMelodicFourOpCount(t *testing.T) {
	metas := []adlMeta{
		{},
		{ID0: 1, ID1: 1},
		{ID0: 2, ID1: 3, Flags: FlagPseudo4op},
		{ID0: 4, ID1: 5, Flags: Flag4op},
		{ID0: 6, ID1: 7, Flags: Flag4op | FlagPseudo4op},
	}
	first := ADLMidiBank{}
	first.Programs[0], first.Programs[1], first.Programs[2] = 1, 2, 3
	first.Programs[3], first.Programs[4] = 3, 4
	other := ADLMidiBank{}
	other.Programs[0] = 3
	banks := []adlBank{{Melos: []ADLMidiBank{first, other}, Drums: []ADLMidiBank{other}}, {}}

	fourOp, total, err := default4OpCount(banks, metas, 0)
	if err != nil || fourOp != 2 || total != 5 {
		t.Fatalf("count = %d/%d, error %v; want 2/5", fourOp, total, err)
	}
	fourOp, total, err = default4OpCount(banks, metas, 1)
	if err != nil || fourOp != 0 || total != 0 {
		t.Fatalf("empty bank count = %d/%d, error %v; want 0/0", fourOp, total, err)
	}
	for _, index := range []int{-1, len(banks)} {
		if _, _, err := default4OpCount(banks, metas, index); err == nil {
			t.Errorf("bank index %d: expected error", index)
		}
	}
	banks[0].Melos[0].Programs[7] = 99
	fourOp, total, err = default4OpCount(banks, metas, 0)
	if err == nil || !strings.Contains(err.Error(), "program 7") || fourOp != 0 || total != 0 {
		t.Fatalf("invalid metadata: count %d/%d, error %v", fourOp, total, err)
	}
}

func TestDefaultMelodicFourOpCountBuiltinMatchesExistingHeuristic(t *testing.T) {
	if ErrBuiltin != nil {
		t.Fatal(ErrBuiltin)
	}
	for index, bank := range banks {
		var wantFourOp, wantTotal uint
		if len(bank.Melos) != 0 {
			for _, id := range bank.Melos[0].Programs {
				if id == 0 || int(id) >= len(adlMetas) {
					continue
				}
				wantTotal++
				meta := adlMetas[id]
				if meta.ID0 != meta.ID1 && !IsPseudo4Op(meta.Flags) {
					wantFourOp++
				}
			}
		}
		fourOp, total, err := Melodic4OpCount(index)
		if err != nil || fourOp != wantFourOp || total != wantTotal {
			t.Errorf("bank %d: count %d/%d, error %v; want %d/%d", index, fourOp, total, err, wantFourOp, wantTotal)
		}
	}
}
