// duh ADL midi driver for OPL3
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

package model

type Native struct{ Generic }

func (n *Native) Volume(volume *OPLModel) {
	volume.scale(nativeVolume(volume))
}

// nativeVolume is shared by the native and RSXX models.
func nativeVolume(volume *OPLModel) int {
	exp := volume.Get(OPLChanExpression)
	vel := volume.Get(OPLVelocity)
	vol := volume.Get(OPLChanVolume)
	mv := volume.Get(OPLMasterVolume)

	// 4096766 = (127 * 127 * 127) * 2
	return min(vel*vol*exp*mv/4_096_766, 63)
}
