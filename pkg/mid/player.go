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

import "time"

// MIDIPlayer combines file sequencing and synth playback.
type MIDIPlayer struct {
	*Sequencer
	*Synth
}

// NewMIDIPlayer creates a file player and its OPL synth.
func NewMIDIPlayer(cardCount, quadOpsCount, flags int) *MIDIPlayer {
	synth := NewSynth(cardCount, quadOpsCount, flags)
	return &MIDIPlayer{Sequencer: NewSequencer(synth), Synth: synth}
}

// NoteOn starts a note and marks playback active if a voice was allocated.
func (m *MIDIPlayer) NoteOn(channel uint32, note, velocity uint8) {
	if m.Synth.playbackNoteOn(channel, note, velocity) {
		m.Sequencer.pos.cur.isPlaying = true
	}
}

// Tick advances the sequencer and, while playing, the synth.
func (m *MIDIPlayer) Tick(dt time.Duration) time.Duration {
	wasPlaying := m.pos.cur.isPlaying
	wait := m.Sequencer.Tick(dt)
	if wasPlaying {
		m.Synth.Advance(dt)
	}
	return wait
}
