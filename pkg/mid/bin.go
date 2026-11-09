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

import "encoding/binary"

func readBEInt(data []byte, nbytes int) (x uint32) {
	for i := range nbytes {
		x = (x << 8) + uint32(data[i])
	}

	return
}

func u16(data []byte) uint16 {
	return binary.LittleEndian.Uint16(data)
}

func u32(data []byte) uint32 {
	return binary.LittleEndian.Uint32(data)
}

func U16(data []byte) uint16 {
	return binary.BigEndian.Uint16(data)
}

func U32(data []byte) uint32 {
	return binary.BigEndian.Uint32(data)
}
