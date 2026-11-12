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

func TestEventMethodsUpdateChannelState(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("test")

	m.ProgramChange(base, 42)
	m.Controller(base, 7, 91)
	m.Controller(base, 10, 23)
	m.ChannelPressure(base, 76)
	m.PitchBend(base, 9000)
	m.NoteAftertouch(base, 60, 55)

	ch := m.mchans[base]
	if ch.patch != 42 || ch.vol != 91 || ch.panning != 23 {
		t.Fatalf("program/volume/pan = %d/%d/%d, want 42/91/23", ch.patch, ch.vol, ch.panning)
	}
	if ch.aftertouch != 76 {
		t.Fatalf("channel pressure = %d, want 76", ch.aftertouch)
	}
	if ch.bend != 808 {
		t.Fatalf("pitch bend = %v, want 808", ch.bend)
	}
	if ch.noteAftertouch[60] != 55 || !ch.noteTouchEnable {
		t.Fatalf("poly pressure state = %d, in-use %v; want 55, true", ch.noteAftertouch[60], ch.noteTouchEnable)
	}
}

func TestAllSoundsOffImmediatelyClearsNotesAndPedalHolds(t *testing.T) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		t.Fatal(err)
	}

	m.NoteOn(base, 60, 100)
	if len(m.mchans[base].activeNotes) == 0 {
		t.Fatal("NoteOn did not create an active note")
	}

	m.Controller(base, 64, 127)
	m.Controller(base, 120, 0)

	if got := len(m.mchans[base].activeNotes); got != 0 {
		t.Fatalf("active notes after CC120 = %d, want 0", got)
	}
	for i, ch := range m.achans {
		for loc := range ch.users {
			if loc.mchan == uint16(base) {
				t.Fatalf("CC120 left channel %d note %d registered", i, loc.note)
			}
		}
	}
}

func BenchmarkNoteOnOff(b *testing.B) {
	m := NewMIDIPlayer(1, 0, 0)
	base := m.DeviceSelect("test")
	if err := m.initADLChannels(); err != nil {
		b.Fatal(err)
	}

	m.NoteOn(base, 60, 100)
	if _, ok := m.mchans[base].activeNotes[60]; !ok {
		b.Fatal("NoteOn did not create an active note")
	}
	m.NoteOff(base, 60)
	if _, ok := m.mchans[base].activeNotes[60]; ok {
		b.Fatal("NoteOff did not remove the active note")
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.NoteOn(base, 60, 100)
		m.NoteOff(base, 60)
	}
}
