// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package instrument

import "github.com/erik-adelbert/duh/internal/tracker/components/core"

type NoteBehavior struct {
	NNA uint8 // New Note Action
	DCT uint8 // Duplicate Check Type
	DCA uint8 // Duplicate Check Action
}

type Filter struct {
	CutOff    uint8 // IFC
	Resonance uint8 // IFR
}

type PitchPan struct {
	Separation uint8 // PPS
	Center     uint8 // PPC
}

type Mappings struct {
	Keys  [128]uint8
	Notes [128]uint8
}

type Metadata struct {
	Name     string
	Filename string
}

type Midi struct {
	Bank    uint16
	Prog    uint8
	Channel uint8
	DrumKey uint8
}

type Params struct {
	core.Span[uint8]
	Sustain core.Span[uint8]
}
