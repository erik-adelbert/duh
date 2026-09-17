// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package voice

// Pitch and period control
type PitchControl struct {
	Period     int32
	C4Speed    int32
	Portamento struct {
		Dest  int32
		Slide int32
	}
}

// Effect memory for vibrato, tremolo, panbrello
type EffectMemory struct {
	AutoVib struct {
		Depth int32
		Pos   uint32
	}
	Vibrato   fxMem
	Tremolo   fxMem
	Panbrello fxMem
}

type fxMem struct {
	Pos   uint32
	Type  uint8
	Speed uint8
	Depth uint8
}

// Note and instrument state
type NoteState struct {
	Note          uint8
	NNA           uint8
	NewNote       uint8
	NewInstrument uint8
	RowNote       uint8
	RowInstrument uint8
}

// Command and effect tracking
type CommandMemory struct {
	Command         uint8
	Arpeggio        uint8
	VolumeSlide     uint8
	FineVolUpDown   uint8
	PortaUpDown     uint8
	FinePortaUpDown uint8
	PanSlide        uint8
	ChnVolSlide     uint8
	CmdEx           uint8
	VolParam        uint8
	Tempo           uint8
	Offset          struct{ Lo, Hi uint8 }
	Row             struct {
		Command uint8
		Param   uint8
		VolCmd  uint8
		Volume  uint8
	}
}

// Filter and sound shaping
type FilterState struct {
	CutOff    uint8
	Resonance uint8
}

// Retrigger, tremor, pattern loop
type PatternControl struct {
	Retrig pctrl
	Tremor pctrl
	Loop   pctrl // Loop, LoopCount
}

type pctrl struct {
	Count, Param uint8
}
