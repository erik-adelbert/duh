// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package voice

import (
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
)

// ChannelRefs holds references to sample and instrument data.
type Instrument struct {
	Sample id.ID
	Instru id.ID
}

// EnvelopeState holds envelope positions.
type Envelope struct {
	core.Balance[uint32]
	Pitch uint32
}

// MacroAutomation holds macro/automation state.
type Automation struct {
	ActiveMacro uint8
}

// Defaults holds default channel settings.
type Defaults struct {
	core.Balance[uint32]
	Flags uint32
	Name  string
}

// SwingRandomization holds swing/randomization state.
type Swing core.Balance[int16]

// VUMeterOutput holds VU meter/output state.
type VUMeter struct {
	core.Stereo[uint8]
	Meter uint32
}

type PatchID = int // PatchID is a hashed identifier for a mixer patch.

// HashPatchID computes a PatchID from the given parameters.
func HashPatchID(fastmono, stereo, bit16, ramp, filter, linear, spline, fir bool) PatchID {
	h := 0
	// Channels: 0 = Mono, 1 = FastMono, 2 = Stereo
	ch := 0

	switch {
	case fastmono:
		ch = 1
	case stereo:
		ch = 2
	}

	h |= ch

	// Bit16: 0 = 8-bit, 1 = 16-bit
	if bit16 {
		h |= 1 << 2
	}

	// Ramp: 0 = no, 1 = yes
	if ramp {
		h |= 1 << 3
	}

	// Filter: 0 = no, 1 = yes
	if filter {
		h |= 1 << 4
	}

	var mode int

	switch {
	case linear:
		mode = 1
	case spline:
		mode = 2
	case fir:
		mode = 3
	}

	h |= mode << 5

	return h
}
