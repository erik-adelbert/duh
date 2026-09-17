// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import "github.com/erik-adelbert/duh/internal/help"

// Log tables for pre-amp
// We don't want the tracker to get too loud
var PreAmpLUT = [16]uint{
	0x60, 0x60, 0x60, 0x70, // 0-7
	0x80, 0x88, 0x90, 0x98, // 8-15
	0xA0, 0xA4, 0xA8, 0xB0, // 16-23
	0xB4, 0xB8, 0xBC, 0xC0, // 24-31
}
var PreAmpAGCLUT = [16]uint{
	0x60, 0x60, 0x60, 0x60,
	0x68, 0x70, 0x78, 0x80,
	0x84, 0x88, 0x8C, 0x90,
	0x94, 0x98, 0x9C, 0xA0,
}

func Attenuation(index int) uint {
	index = help.Clamp(index, 0, 31) / 2

	return PreAmpLUT[index]
}

func AGCAttenuation(index int) uint {
	index = help.Clamp(index, 0, 31) / 2

	return PreAmpAGCLUT[index]
}
