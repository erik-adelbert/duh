// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mid

import (
	"testing"
	"time"
)

func voiceTimingSynth() (*Synth, location) {
	s := NewSynth(1, 0, 0)
	ch := s.DeviceSelect("")
	loc := location{mchan: uint16(ch), note: 60}
	s.mchans[ch].activeNotes[loc.note] = &noteInfo{phys: map[int]uint16{0: 0}}
	return s, loc
}

func TestPrepareForNewNoteTiming(t *testing.T) {
	for _, tt := range []struct {
		name        string
		delay, left time.Duration
		ins         int
		wantKept    bool
	}{
		{"just before 70ms", 70*time.Millisecond - time.Nanosecond, 0, 0, true},
		{"at 70ms", 70 * time.Millisecond, 0, 0, false},
		{"at 20s", 70 * time.Millisecond, 20 * time.Second, 0, false},
		{"over 20s", 70 * time.Millisecond, 20*time.Second + time.Nanosecond, 0, true},
		{"different instrument", 0, 30 * time.Second, 1, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, loc := voiceTimingSynth()
			s.achans[0].users[loc] = locData{vibDelay: tt.delay, KONDecay: tt.left}
			s.PrepareForNewNote(0, tt.ins)
			_, got := s.achans[0].users[loc]
			if got != tt.wantKept {
				t.Errorf("kept = %t, want %t", got, tt.wantKept)
			}
		})
	}
}

func TestMoveOrKillTiming(t *testing.T) {
	for _, tt := range []struct {
		name        string
		delay, left time.Duration
		wantMoved   bool
	}{
		{"just before 200ms", 200*time.Millisecond - time.Nanosecond, 0, true},
		{"at 200ms below 10s", 200 * time.Millisecond, 10*time.Second - time.Nanosecond, false},
		{"at 200ms and 10s", 200 * time.Millisecond, 10 * time.Second, true},
		{"over 200ms and 10s", 201 * time.Millisecond, 11 * time.Second, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, loc := voiceTimingSynth()
			data := locData{ins: 0}
			s.achans[0].users[loc] = data
			s.achans[1].users[location{note: 61}] = locData{ins: 0, vibDelay: tt.delay, KONDecay: tt.left}
			s.MoveOrKill(0, loc, data)
			_, got := s.achans[1].users[loc]
			if got != tt.wantMoved {
				t.Errorf("moved = %t, want %t", got, tt.wantMoved)
			}
			if _, remains := s.achans[0].users[loc]; remains {
				t.Error("original channel still contains note")
			}
		})
	}
}

func TestChannelScoreEvacuationTiming(t *testing.T) {
	s, loc := voiceTimingSynth()
	s.achans[0].users[loc] = locData{ins: 0}
	other := location{note: 61}
	s.achans[1].users[other] = locData{ins: 0, vibDelay: 200 * time.Millisecond}
	atBoundary := s.ChannelScore(0, 0, 0)
	s.achans[1].users[other] = locData{ins: 0, vibDelay: 200*time.Millisecond - time.Nanosecond}
	beforeBoundary := s.ChannelScore(0, 0, 0)
	if beforeBoundary-atBoundary != 4 {
		t.Errorf("score increase before 200ms = %d, want 4", beforeBoundary-atBoundary)
	}
}

func TestSharedNotesArpeggiate(t *testing.T) {
	s := NewSynth(1, 0, 0)
	ch := s.DeviceSelect("")
	s.setCMFInstruments([]adlData{{ModulatorE862: 0x21, CarrierE862: 0x21}})
	s.useCMFBank()

	// Force notes to compete for the same regular channel.
	for i := 1; i < len(s.OPL.quads); i++ {
		s.OPL.quads[i] = PercSlave
	}
	s.NoteOn(ch, 60, 100)
	s.Advance(10 * time.Millisecond)
	s.NoteOn(ch, 64, 100)

	first := location{mchan: uint16(ch), note: 60}
	second := location{mchan: uint16(ch), note: 64}
	if len(s.achans[0].users) != 2 {
		t.Fatalf("shared users = %d, want 2", len(s.achans[0].users))
	}

	frequencies := make(map[[2]byte]bool)
	for range 6 {
		s.UpdateArpeggio(10 * time.Millisecond)
		state, ok := s.OPL.Cards[0].State()
		if !ok {
			t.Fatal("OPL state unavailable")
		}
		frequencies[[2]byte{state.Registers[0][0xa0], state.Registers[0][0xb0]}] = true
	}
	if len(frequencies) != 2 {
		t.Errorf("arpeggio used %d pitches, want 2", len(frequencies))
	}

	s.NoteOff(ch, first.note)
	if len(s.achans[0].users) != 1 {
		t.Fatalf("users after note off = %d, want 1", len(s.achans[0].users))
	}
	if _, ok := s.achans[0].users[second]; !ok {
		t.Fatal("remaining note lost its channel")
	}
}
