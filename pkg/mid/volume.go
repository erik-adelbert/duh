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

package mid

import "github.com/erik-adelbert/duh/pkg/mid/model"

// newVolumeModel returns the volume and frequency model for a Vol* id.
// Unknown ids fall back to the generic model.
func newVolumeModel(id int) model.Volume {
	if id <= 0 || id >= VolModelCount {
		return &model.Generic{}
	}

	return [...]model.Volume{
		VolLinear:      &model.Generic{},
		VolNative:      &model.Native{},
		VolDMX:         &model.Dmx{},
		VolApogee:      &model.Apogee{},
		VolWin9x:       &model.SB16{},
		VolDMXFixed:    &model.DmxFixed{},
		VolApogeeFixed: &model.ApogeeFixed{},
		VolAIL:         &model.Ail{},
		VolWin9xFM:     &model.Win9x{},
		VolHMI:         &model.Hmi{},
		VolHMIOld:      &model.HmiOld{},
		VolMSAdlib:     &model.Adlib{},
		VolIMFCreator:  &model.ImfCreator{},
		VolOConnel:     &model.OConnel{},
	}[id]
}

// bankVolumeModel returns the volume model id declared by a bank's setup.
func bankVolumeModel(bank uint32) int {
	if int(bank) >= len(banks) {
		return VolLinear
	}

	return 1 + int(banks[bank].Setup&0xFF)
}

func VolumeModels() []string {
	return []string{
		"From Bank (default)",
		"Generic",
		"Native",
		"DMX",
		"Apogee",
		"Win9x",
		"DMX Fixed",
		"Apogee Fixed",
		"AIL",
		"Win9x FM",
		"HMI",
		"HMI Old",
		"MS Adlib",
		"IMF Creator",
		"O'Connel",
	}
}
