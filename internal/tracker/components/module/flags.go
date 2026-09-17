// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package module

type Flags uint32

const (
	Oversample Flags = 1 << iota
	NoiseReduction
	Reverb
	BassBoost
	Surround
)
const (
	WithMIDISetup Flags = 0x001
	FastSlides    Flags = 0x002
	LinearSlides  Flags = 0x010
	PatternLoop   Flags = 0x020
	Step          Flags = 0x040
	Paused        Flags = 0x080
	Fading        Flags = 0x100
	Ended         Flags = 0x200
	GlobalFading  Flags = 0x400
	FirstTick     Flags = 0x1000
	AmigaLimits   Flags = 0x10000
)
