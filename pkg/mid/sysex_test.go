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

package mid

import "testing"

func TestDUHSysExSelectsADLBank(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("test")
	m.mchans[base].vol = 73
	m.mchans[base].vol0 = 73

	// F0 7D "DUH" version 1, select bank, bank 78, F7.
	if !m.SysEx(0, []byte{0x7D, 'D', 'U', 'H', 0x01, 0x01, 0x00, 0x4E, 0xF7}) {
		t.Fatal("valid bank-selection SysEx was rejected")
	}
	if m.bank != 78 {
		t.Fatalf("selected bank = %d, want 78", m.bank)
	}
	if got := m.mchans[base].vol; got != 73 {
		t.Fatalf("bank selection changed current channel volume to %d, want 73", got)
	}
}

func TestDUHSysExRejectsInvalidBankWithoutChangingSelection(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	initial := m.bank

	if m.SysEx(0, []byte{0x7D, 'D', 'U', 'H', 0x01, 0x01, 0x7F, 0x7F, 0xF7}) {
		t.Fatal("out-of-range bank SysEx was accepted")
	}
	if m.bank != initial {
		t.Fatalf("invalid message changed bank to %d, want %d", m.bank, initial)
	}

	if m.SysEx(0, []byte{0x7D, 'D', 'U', 'H', 0x02, 0x01, 0x00, 0x4E, 0xF7}) {
		t.Fatal("unsupported protocol version was accepted")
	}
	if m.bank != initial {
		t.Fatalf("unsupported message changed bank to %d, want %d", m.bank, initial)
	}

	if m.SysEx(0, []byte{0x7D, 'D', 'U', 'H', 0x01, 0x01, 0x80, 0x4E, 0xF7}) {
		t.Fatal("non-7-bit bank data was accepted")
	}
	if m.bank != initial {
		t.Fatalf("invalid data changed bank to %d, want %d", m.bank, initial)
	}
}
