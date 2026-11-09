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

package main

import mid "github.com/erik-adelbert/duh/pkg/mid"

type idef [11]byte

func (i idef) amvib(op int) byte {
	if op == 1 {
		return i[0]
	}

	return i[1]
}

func (i idef) atdec(op int) byte {
	if op == 1 {
		return i[2]
	}

	return i[3]
}

func (i idef) susrel(op int) byte {
	if op == 1 {
		return i[4]
	}

	return i[5]
}

func (i idef) wave(op int) byte {
	if op == 1 {
		return i[6]
	}

	return i[7]
}

func (i idef) ksltl(op int) byte {
	if op == 1 {
		return i[8]
	}

	return i[9]
}

func (i idef) fb(_ int) byte {
	return i[10]
}

func toOPs(d idef, ops []mid.OP, off int) {
	for i := range 2 {
		ops[off+i].E862 = uint32(d.wave(i))<<24 | uint32(d.susrel(i))<<16
		ops[off+i].E862 |= uint32(d.atdec(i))<<8 | uint32(d.amvib(i))
		ops[off+i].D40 = uint32(d.ksltl(i))
	}
}
