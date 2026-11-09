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
	"path/filepath"
	"testing"
	"time"
)

func TestAsTables(t *testing.T) {
	db := NewBankDB()
	db.Ops = []*OP{
		{E862: 0x11223344, D40: 0x55},
		{E862: 0x66778899, D40: 0xAA},
		{E862: 0xAABBCCDD, D40: 0x11},
		{E862: 0xEEFF0011, D40: 0x22},
	}

	twoOp := NewInsEntry()
	twoOp.Ops[0], twoOp.Ops[1] = 0, 1
	twoOp.Note.Off0 = -2
	twoOp.FBConn = 0x0005
	twoOp.VelOff = 3
	twoOp.KON = 10 * time.Millisecond
	twoOp.KOFF = 20 * time.Millisecond

	pseudo4Op := NewInsEntry()
	pseudo4Op.Flags = FlagPseudo4op | FlagFixedTone
	pseudo4Op.Ops = [4]int{0, 1, 2, 3}
	pseudo4Op.Note.Off0 = 1
	pseudo4Op.Note.Off1 = -4
	pseudo4Op.SecVoice = 2
	pseudo4Op.DrumKey = 36
	pseudo4Op.FBConn = 0x0703

	blank := NewInsEntry()
	blank.Flags = FlagSilence
	db.Ins = []*InsEntry{twoOp, pseudo4Op, blank}

	melodic := NewMidiBank(0, 0)
	melodic.Ins[0] = 0
	melodic.Ins[1] = 1
	melodic.Ins[2] = 2
	drums := NewMidiBank(0, 1)
	drums.Ins[36] = 1
	db.Midis = []*MidiBank{melodic, drums}
	db.Banks = []*BankEntry{{
		ID:    7,
		Name:  "test bank",
		Melos: []int{0},
		Drums: []int{1},
	}}

	got, err := db.AsTables()
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Instruments) != 4 {
		t.Fatalf("got %d ADL instruments, want 4 (including silent)", len(got.Instruments))
	}
	if got.Instruments[1] != (adlData{
		ModulatorE862: 0x11223344,
		CarrierE862:   0x66778899,
		Modulator40:   0x55,
		Carrier40:     0xAA,
		FeedConn:      5,
		Finetune:      -2,
	}) {
		t.Errorf("2-op instrument = %+v", got.Instruments[1])
	}
	if got.Metas[1].ID0 != got.Metas[1].ID1 || got.Metas[1].VelocityOffset != 3 {
		t.Errorf("2-op meta = %+v, want identical IDs and velocity offset 3", got.Metas[1])
	}
	if meta := got.Metas[2]; meta.ID0 == meta.ID1 ||
		!Is4Op(meta.Flags) || meta.Tone != 36 ||
		meta.VelocityOffset != 0 || meta.SecondFinetune != 2 {
		t.Errorf("pseudo-4op meta = %+v", meta)
	}
	if got.Instruments[3].Finetune != -4 || got.Instruments[3].FeedConn != 7 {
		t.Errorf("second pseudo-4op voice = %+v", got.Instruments[3])
	}
	if !has(got.Metas[0].Flags, FlagSilence) {
		t.Errorf("silent meta flags = %b, want FlagSilence set", got.Metas[0].Flags)
	}

	bank := got.Banks[0]
	if bank.ID != 7 || bank.name != "test bank" ||
		bank.Melos[0].Programs[0] != 1 ||
		bank.Melos[0].Programs[1] != 2 ||
		bank.Melos[0].Programs[2] != 0 ||
		bank.Drums[0].Programs[36] != 2 {
		t.Errorf("derived bank mappings incorrect: %+v", bank)
	}

	path := filepath.Join(t.TempDir(), "bankdb.gob")
	if err := db.Dump(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	roundTripped, err := loaded.AsTables()
	if err != nil {
		t.Fatal(err)
	}
	if got := roundTripped.Banks[0].Melos[0].Programs[1]; got != 2 {
		t.Errorf("round-tripped program mapping = %d, want 2", got)
	}
}

func TestInstrumentTypeFlags(t *testing.T) {
	tests := []struct {
		name         string
		flags        int
		fourOp       bool
		pseudoFourOp bool
	}{
		{name: "zero-valued 2op"},
		{name: "2op", flags: Flag2op, fourOp: false},
		{name: "fixed-note 2op", flags: Flag2op | FlagFixedTone, fourOp: false},
		{name: "4op", flags: Flag4op, fourOp: true},
		{name: "pseudo-4op", flags: FlagPseudo4op, fourOp: true, pseudoFourOp: true},
		{name: "combined pseudo-4op", flags: Flag4op | FlagPseudo4op, fourOp: true, pseudoFourOp: true},
		{name: "fixed-note pseudo-4op", flags: FlagPseudo4op | FlagFixedTone, fourOp: true, pseudoFourOp: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is4Op(tt.flags); got != tt.fourOp {
				t.Errorf("IsFourOp(%#x) = %t, want %t", tt.flags, got, tt.fourOp)
			}
			if got := IsPseudo4Op(tt.flags); got != tt.pseudoFourOp {
				t.Errorf("IsPseudoFourOp(%#x) = %t, want %t", tt.flags, got, tt.pseudoFourOp)
			}
		})
	}
}

func TestBuiltinADLDatabaseLoads(t *testing.T) {
	if ErrBuiltin != nil {
		t.Fatal(ErrBuiltin)
	}
	if len(banks) == 0 || len(adlMetas) == 0 || len(adlIns) == 0 {
		t.Fatalf("built-in tables are empty: %d banks, %d metas, %d instruments",
			len(banks), len(adlMetas), len(adlIns))
	}
	if len(banks[0].Melos) == 0 {
		t.Fatal("first built-in bank has no melodic MIDI bank")
	}
}

func TestBuiltinGENMIDIPseudoFourOp(t *testing.T) {
	for _, bank := range banks {
		if bank.name != "WOPL (GENMIDI)" {
			continue
		}
		if bank.Setup != VolLinear {
			t.Errorf("GENMIDI setup = %#x, want generic volume and shallow modulation", bank.Setup)
		}
		if len(bank.Melos) == 0 {
			t.Fatal("GENMIDI has no melodic bank")
		}
		for _, id := range bank.Melos[0].Programs {
			if int(id) >= len(adlMetas) {
				t.Fatalf("invalid meta ID %d", id)
			}
			meta := adlMetas[id]
			if IsPseudo4Op(meta.Flags) && meta.ID0 != meta.ID1 {
				return
			}
		}
		t.Fatal("GENMIDI has no two-voice pseudo-4op patches")
	}
	t.Fatal("built-in GENMIDI bank not found")
}

func TestBuiltinBank38UsesWOPLXDrumKey(t *testing.T) {
	const bankIndex = 38
	if bankIndex >= len(banks) {
		t.Fatalf("built-in bank %d is missing", bankIndex)
	}

	bank := banks[bankIndex]
	if bank.name != "AIL (SimFarm, SimHealth) :4op:" {
		t.Fatalf("bank %d = %q, want SimFarm/SimHealth 4-op bank", bankIndex, bank.name)
	}
	if len(bank.Drums) == 0 {
		t.Fatal("bank 38 has no percussion MIDI bank")
	}

	metaID := bank.Drums[0].Programs[38]
	if metaID == 0 || int(metaID) >= len(adlMetas) {
		t.Fatalf("bank 38 percussion patch 38 has invalid meta ID %d", metaID)
	}
	if got := adlMetas[metaID].Tone; got != 48 {
		t.Errorf("bank 38 percussion patch 38 fixed tone = %d, want DRUM_KEY 48", got)
	}
}

func TestAsTablesRejectsBrokenReferences(t *testing.T) {
	db := NewBankDB()
	db.Ops = []*OP{{}}
	ins := NewInsEntry()
	ins.Ops[0], ins.Ops[1] = 2, 0
	db.Ins = []*InsEntry{ins}
	db.Banks = []*BankEntry{{ID: 0, Melos: []int{0}}}
	db.Midis = []*MidiBank{NewMidiBank(0, 0)}
	db.Midis[0].Ins[0] = 0

	if _, err := db.AsTables(); err == nil {
		t.Fatal("DeriveADL succeeded with an invalid operator reference")
	}
}

func TestGetAdlMetaIDUsesDerivedMidiBanks(t *testing.T) {
	oldBanks := banks
	defer func() { banks = oldBanks }()

	defaultMelodic := ADLMidiBank{}
	defaultMelodic.Programs[0] = 4
	selectedMelodic := ADLMidiBank{MSB: 3, LSB: 2}
	selectedMelodic.Programs[0] = 7
	drumBank := ADLMidiBank{}
	drumBank.Programs[36] = 9
	banks = []adlBank{{
		Melos: []ADLMidiBank{defaultMelodic, selectedMelodic},
		Drums: []ADLMidiBank{drumBank},
	}}

	decoder := &Synth{bank: 0}
	if got := decoder.GetAdlMetaID(0, nil); got != 4 {
		t.Errorf("default melodic meta ID = %d, want 4", got)
	}
	channel := &midiChan{hasBank: true, bank: bank{MSB: 3, LSB: 2}}
	if got := decoder.GetAdlMetaID(0, channel); got != 7 {
		t.Errorf("selected melodic meta ID = %d, want 7", got)
	}
	if got := decoder.GetAdlMetaID(128+36, nil); got != 9 {
		t.Errorf("drum meta ID = %d, want 9", got)
	}
}
