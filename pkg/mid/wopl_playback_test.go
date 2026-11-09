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
	"testing"

	"github.com/erik-adelbert/duh/pkg/mid/model"
)

func TestWOPLBankModeAndRhythmPlayback(t *testing.T) {
	oldBanks, oldMetas, oldIns, oldSelect := banks, adlMetas, adlIns, BankSelect
	t.Cleanup(func() {
		banks, adlMetas, adlIns, BankSelect = oldBanks, oldMetas, oldIns, oldSelect
	})
	BankSelect = 0
	adlIns = []adlData{{}, {ModulatorE862: 0x01010111, CarrierE862: 0x01010122}}
	adlMetas = []adlMeta{{Flags: FlagSilence}, {ID0: 1, ID1: 1}}
	drums := ADLMidiBank{}
	for i := range 5 {
		adlMetas = append(adlMetas, adlMeta{
			ID0: 1, ID1: 1, Flags: FlagRMBassDrum + i*8,
			Tone: uint8(10 + i),
		})
		drums.Programs[35+i] = uint16(2 + i)
	}
	melodic := ADLMidiBank{}
	melodic.Programs[0] = 1
	banks = []adlBank{
		{Melos: []ADLMidiBank{melodic}},
		{Setup: VolAIL | 7<<8, Melos: []ADLMidiBank{melodic}, Drums: []ADLMidiBank{drums}},
	}
	m := NewMIDIPlayer(2, 0, 0)
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		t.Fatal(err)
	}
	m.NoteOn(base, 60, 100)
	if err := m.SetBank(1); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.mchans[base].activeNotes[60]; !ok {
		t.Fatal("switch stopped a note on an unaffected channel")
	}
	if _, ok := m.OPL.vm.(*model.Ail); !ok {
		t.Fatalf("bank volume model = %T, want AIL", m.OPL.vm)
	}
	if vol, bend := m.defaults(); vol != 127 || bend != 12 {
		t.Errorf("MT-32 defaults = %d/%d", vol, bend)
	}
	for card := range 2 {
		if got := m.OPL.bds[card] & 0xe0; got != 0xe0 {
			t.Errorf("card %d mode = %#x, want %#x", card, got, 0xe0)
		}
		for i := range 3 {
			if got := m.OPL.quads[card*23+6+i]; got != PercSlave {
				t.Errorf("card %d channel %d category = %d", card, i+6, got)
			}
		}
	}
	for i := range 5 {
		note := uint8(35 + i)
		m.NoteOn(base+9, note, 100)
		info := m.mchans[base+9].activeNotes[note]
		if info == nil || len(info.phys) != 1 {
			t.Fatalf("drum %d did not allocate one channel: %+v", note, info)
		}
		if info.tone != int16(10+i) {
			t.Errorf("drum %d tone = %d, want %d", note, info.tone, 10+i)
		}
		for phys := range info.phys {
			if got := m.OPL.quads[phys]; got != int8(PercBD+i) {
				t.Errorf("drum %d category = %d, want %d", note, got, PercBD+i)
			}
			if m.OPL.bds[phys/23]&(0x10>>i) == 0 {
				t.Errorf("drum %d was not keyed on", note)
			}
			state, ok := m.OPL.Cards[phys/23].State()
			if !ok {
				t.Fatal("OPL state unavailable")
			}
			for slot, value := range []uint8{0x11, 0x22} {
				op := m.OPL.operator(phys%23, slot)
				if op != 0xfff && state.Registers[0][0x20+op] != value {
					t.Errorf("drum %d operator %d register = %#x, want %#x", note, slot, state.Registers[0][0x20+op], value)
				}
			}
		}
	}
	banks[0].Setup = VolWin9x
	m.SetVolumeModel(1)
	if err := m.SetBank(0); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.OPL.vm.(*model.Generic); !ok {
		t.Errorf("explicit volume model was not preserved: %T", m.OPL.vm)
	}
	if len(m.mchans[base+9].activeNotes) != 0 {
		t.Fatal("switch away left rhythm notes active")
	}
	if _, ok := m.mchans[base].activeNotes[60]; !ok {
		t.Fatal("switch away stopped a note on an unaffected channel")
	}
	for card := range 2 {
		if got := m.OPL.bds[card]; got != 0 {
			t.Errorf("card %d BD = %#x after switching away", card, got)
		}
	}
}

func TestRhythmOperatorMapping(t *testing.T) {
	o := newOPL3(1, 0, 0)
	want := [5][2]uint16{{0x10, 0x13}, {0xfff, 0x14}, {0x12, 0xfff}, {0xfff, 0x15}, {0x11, 0xfff}}
	for i, pair := range want {
		for slot := range 2 {
			if got := o.operator(18+i, slot); got != pair[slot] {
				t.Errorf("percussion %d operator %d = %#x, want %#x", i, slot, got, pair[slot])
			}
		}
	}
	o.cmf = true
	if o.operator(19, 0) != 0x14 || o.operator(21, 0) != 0x15 {
		t.Error("CMF snare and cymbal operator mappings changed")
	}
}

func TestBankSwitchStopsOnlyAffectedVoices(t *testing.T) {
	oldBanks, oldMetas, oldSelect := banks, adlMetas, BankSelect
	t.Cleanup(func() { banks, adlMetas, BankSelect = oldBanks, oldMetas, oldSelect })
	BankSelect = 0
	adlMetas = []adlMeta{{Flags: FlagSilence}, {Flags: FlagRMSnare}}
	drums := ADLMidiBank{}
	drums.Programs[42] = 1
	banks = []adlBank{{}, {Drums: []ADLMidiBank{drums}}}
	m := NewMIDIPlayer(1, 0, ModeHighTremolo)
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		t.Fatal(err)
	}
	loc := location{mchan: uint16(base), note: 60}
	m.mchans[base].activeNotes[60] = &noteInfo{
		instrument: instrument{Meta: 0}, phys: phys{6: 0},
	}
	m.achans[6].users[loc] = locData{ins: 0}
	held := location{mchan: uint16(base), note: 61}
	m.achans[7].users[held] = locData{ins: 0, sustained: HoldPedal}
	unaffected := location{mchan: uint16(base), note: 62}
	m.mchans[base].activeNotes[62] = &noteInfo{
		instrument: instrument{Meta: 0}, phys: phys{9: 0},
	}
	m.achans[9].users[unaffected] = locData{ins: 0}

	if err := m.SetBank(1); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.mchans[base].activeNotes[60]; ok || len(m.achans[6].users) != 0 || len(m.achans[7].users) != 0 {
		t.Fatal("affected or sustained notes survived mode switch")
	}
	if _, ok := m.mchans[base].activeNotes[62]; !ok || len(m.achans[9].users) != 1 {
		t.Fatal("unaffected note was stopped")
	}
	if err := m.SetBank(0); err != nil {
		t.Fatal(err)
	}
	if got := m.OPL.bds[0] & 0xe0; got != 0x80 {
		t.Errorf("explicit tremolo mode lost after bank switch: %#x", got)
	}
}

func TestPseudoFourOpPlaybackUsesTwoVoices(t *testing.T) {
	oldBanks, oldMetas, oldIns, oldSelect := banks, adlMetas, adlIns, BankSelect
	t.Cleanup(func() { banks, adlMetas, adlIns, BankSelect = oldBanks, oldMetas, oldIns, oldSelect })
	BankSelect = 0
	adlIns = []adlData{{}, {ModulatorE862: 1}, {ModulatorE862: 2}}
	adlMetas = []adlMeta{{Flags: FlagSilence}, {
		ID0: 1, ID1: 2, Flags: FlagPseudo4op, SecondFinetune: 0.5,
	}}
	melodic := ADLMidiBank{}
	melodic.Programs[0] = 1
	banks = []adlBank{{Melos: []ADLMidiBank{melodic}}}
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		t.Fatal(err)
	}

	m.NoteOn(base, 60, 100)
	info := m.mchans[base].activeNotes[60]
	if info == nil || len(info.phys) != 2 {
		t.Fatalf("pseudo-4op allocated wrong number of voices: %+v", info)
	}
	for channel, ins := range info.phys {
		if m.OPL.quads[channel] != Regular || ins != 1 && ins != 2 {
			t.Errorf("voice channel %d: category %d, instrument %d", channel, m.OPL.quads[channel], ins)
		}
		tone := 60.0
		if ins == 2 {
			tone += 0.5
		}
		var offset uint32
		want := int(m.OPL.vm.Freq(tone, &offset)) | 0x2000
		m.OPL.Cards[channel/23].Snapshot()
		state, ok := m.OPL.Cards[channel/23].State()
		if !ok {
			t.Fatal("OPL state unavailable")
		}
		reg := Channels[channel%23]
		if got := int(state.Registers[reg>>8][0xa0+reg&0xff]) | int(state.Registers[reg>>8][0xb0+reg&0xff])<<8; got != want {
			t.Errorf("voice %d frequency = %#x, want %#x", ins, got, want)
		}
	}
}

func TestCMFRhythmModeRoutesChannelVoices(t *testing.T) {
	oldBanks, oldMetas, oldIns, oldSelect := banks, adlMetas, adlIns, BankSelect
	t.Cleanup(func() { banks, adlMetas, adlIns, BankSelect = oldBanks, oldMetas, oldIns, oldSelect })
	BankSelect = 0
	adlIns = []adlData{{}, {ModulatorE862: 0x11, CarrierE862: 0x22}}
	adlMetas = []adlMeta{{Flags: FlagSilence}, {ID0: 1, ID1: 1}}
	melodic := ADLMidiBank{}
	melodic.Programs[0] = 1
	banks = []adlBank{{Melos: []ADLMidiBank{melodic}}}
	m := NewMIDIPlayer(1, 0, ModeADLPercussion)
	m.OPL.cmf = true
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		t.Fatal(err)
	}
	m.Controller(base, 103, 1)
	m.NoteOn(base+12, 60, 100)
	info := m.mchans[base+12].activeNotes[60]
	if info == nil || len(info.phys) != 1 {
		t.Fatalf("CMF snare did not allocate a rhythm channel: %+v", info)
	}
	for channel := range info.phys {
		if got := m.OPL.quads[channel]; got != PercSnare {
			t.Errorf("CMF channel category = %d, want snare", got)
		}
		state, ok := m.OPL.Cards[channel/23].State()
		if !ok || state.Registers[0][0x34] != 0x11 {
			t.Errorf("CMF snare modulator register = %#x, want %#x", state.Registers[0][0x34], 0x11)
		}
	}
}
