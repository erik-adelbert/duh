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
	"math"
	"time"
)

type midiChan struct {
	bank
	hasBank bool

	patch  uint8
	vol    uint8
	vol0   uint8 // volume restored by Reset
	expr   uint8
	bright uint8

	vibrato

	aftertouch      uint8
	noteAftertouch  [128]uint8
	noteTouchEnable bool

	panning uint8

	isDrum bool // channel plays drums besides MIDI channel 10

	portamento

	sustain uint8
	isSoft  bool
	pitch
	pitch0 uint8 // bend sensitivity (semitones) restored by Reset

	lastLRPN uint8
	lastMRPN uint8
	hasNRPN  bool

	activeNotes map[uint8]*noteInfo
}

type pitch struct {
	bend     float64
	MSB, LSB int
}

func (p *pitch) Sensitivity() float64 {
	return float64((p.MSB<<7)+p.LSB) / (128 * 8192.0)
}

func (p *pitch) Value() float64 {
	return p.bend * p.Sensitivity()
}

type bank struct {
	LSB, MSB uint8
}

type vibrato struct {
	lvl   uint8
	pos   float64
	vel   float64
	depth float64
	delay time.Duration
}

type portamento struct {
	enabled bool
	lvl     uint16
	src     int8    // last note played on the channel, -1 if none
	rate    float64 // semitones per second, +Inf when disabled
}

// noteInfo holds info about an active note
type noteInfo struct {
	tone int16

	vol uint8

	// pressure is the polyphonic aftertouch, mixed into vibrato
	pressure uint8

	// glide is the tone currently sounding, sliding towards Tone at
	// GlideRate semitones per second (+Inf when the note doesn't glide)
	glide     float64
	glideRate float64

	instrument

	// InsMidi uint8
	// InsMeta uint16
	// kind noteType

	phys

	TTL time.Duration

	// nglide, nxtend int
}

type phys map[int]uint16

type noteType int

const (
	NoteDrum noteType = 1 << iota
	NoteSilent
	NoteExtended
)

type instrument struct {
	Midi uint8
	Meta uint16
}

// Reset puts the channel back to its power-on controllers, like libADLMIDI's
// resetAllControllers.
func (c *midiChan) Reset() {
	c.vol = c.vol0
	c.bright = 127
	c.panning = 64

	c.ResetControllers()
}

// ResetControllers is the CC121 subset of Reset: volume, brightness and
// panning are left alone.
func (c *midiChan) ResetControllers() {
	c.bend = 0

	c.pitch.MSB, c.pitch.LSB = int(c.pitch0), 0

	c.expr = 127
	c.sustain = 0
	c.isSoft = false
	c.aftertouch = 0
	c.noteAftertouch = [128]uint8{}
	c.noteTouchEnable = false
	c.vibrato = vibrato{
		vel:   2 * math.Pi * 5.0,
		depth: 0.5 / 127,
	}
	c.portamento = portamento{src: -1, rate: math.Inf(1)}
}

// HasVibrato reports whether modulation or any aftertouch drives the vibrato.
func (c *midiChan) HasVibrato() bool {
	return c.vibrato.lvl > 0 || c.aftertouch > 0 || c.noteTouchEnable
}

// IsDrum reports whether the channel at index midCh plays percussion.
func (c *midiChan) IsDrum(midCh uint32) bool {
	return midCh&0xF == 9 || c.isDrum
}

func (c *midiChan) IsXGDrum() bool {
	return c.bank.MSB == 0x7E || c.bank.MSB == 0x7F
}
