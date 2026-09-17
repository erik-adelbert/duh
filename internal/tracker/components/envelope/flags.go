// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envelope

type Flags uint32

const (
	EnvVolume       Flags = 0x0001
	EnvVolSustain   Flags = 0x0002
	EnvVolLoop      Flags = 0x0004
	EnvPanning      Flags = 0x0008
	EnvPanSustain   Flags = 0x0010
	EnvPanLoop      Flags = 0x0020
	EnvPitch        Flags = 0x0040
	EnvPitchSustain Flags = 0x0080
	EnvPitchLoop    Flags = 0x0100
	EnvSetPanning   Flags = 0x0200
	EnvFilter       Flags = 0x0400
	EnvVolCarry     Flags = 0x0800
	EnvPanCarry     Flags = 0x1000
	EnvPitchCarry   Flags = 0x2000
)
