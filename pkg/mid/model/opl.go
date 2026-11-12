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

type OPLModel struct {
	state          [8]uint8
	mod, car, drum bool
}

func (o *OPLModel) Set(field OPLField, value int) {
	o.state[field] = uint8(value)
}

func (o *OPLModel) Get(field OPLField) int {
	return int(o.state[field])
}

func (o *OPLModel) EnableMod(value bool) {
	o.mod = value
}

func (o *OPLModel) EnableCar(value bool) {
	o.car = value
}

func (o *OPLModel) EnableDrum(value bool) {
	o.drum = value
}

func (o *OPLModel) IsMod() bool {
	return o.mod
}

func (o *OPLModel) IsCar() bool {
	return o.car
}

func (o *OPLModel) IsDrum() bool {
	return o.drum
}

// scale applies the common total-level attenuation to the enabled operators.
func (o *OPLModel) scale(v int) {
	if o.mod {
		o.Set(OPLModN, 63-v+v*o.Get(OPLModN)/63)
	}

	if o.car {
		o.Set(OPLCarN, 63-v+v*o.Get(OPLCarN)/63)
	}
}

type OPLField int

const (
	OPLVelocity OPLField = iota
	OPLChanVolume
	OPLChanExpression
	OPLMasterVolume
	OPLVoiceMode
	OPLFbconn
	OPLModN
	OPLCarN
)

const (
	OPLVoice_MODE_2op_FM = iota
	OPLVoice_MODE_2op_AM
	OPLVoice_MODE_4op_1_2_FM_FM
	OPLVoice_MODE_4op_1_2_AM_FM
	OPLVoice_MODE_4op_1_2_FM_AM
	OPLVoice_MODE_4op_1_2_AM_AM
	OPLVoice_MODE_4op_3_4_FM_FM
	OPLVoice_MODE_4op_3_4_AM_FM
	OPLVoice_MODE_4op_3_4_FM_AM
	OPLVoice_MODE_4op_3_4_AM_AM
)
