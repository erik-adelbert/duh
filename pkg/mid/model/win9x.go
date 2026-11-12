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

import "math"

type Win9x struct{}

func (w *Win9x) Freq(tone float64, offset *uint32) uint16 {
	*offset = 0

	note := int(math.Trunc(tone))
	if note >= 12 {
		note -= 12
	}

	half := tone - math.Trunc(tone)

	if half > 0.5 {
		note += 1
		half -= 1.0
	}

	pitch := int(math.Trunc(half*4096)) + 8192
	msb := (pitch >> 7) & 0x7F
	lsb := pitch & 0x7F

	pitch = (msb << 9) | (lsb << 2)
	pitch = int(int16(uint16(pitch + 0x8000)))

	// bend = (bendMsb << 9) | (bendLsb << 2);
	// bend = (int16_t)(uint16_t)(bend + 0x8000);

	oct := note / 12
	freq := win9xFreqs[note%12]

	if oct < 5 {
		freq >>= (5 - oct)
	} else if oct > 5 {
		freq <<= (oct - 5)
	}

	freq = 2 * win9xbend(freq, pitch)

	oct = 1
	for freq > 0x3ff {
		freq /= 2
		oct++
	}

	for oct > 7 {
		*offset++
		oct--
	}

	return uint16(oct<<10 | freq)
}

func (w *Win9x) Volume(volume *OPLModel) {
	exp := volume.Get(OPLChanExpression)
	vol := volume.Get(OPLChanVolume)
	mv := volume.Get(OPLMasterVolume)

	v := (vol * exp * mv) / 16129
	v = win9xModels[v>>2]

	set := func(f OPLField) {
		vel := volume.Get(OPLVelocity)

		fv := volume.Get(f)
		fv += v + win9xModels[vel>>2]
		fv = min(fv, 63)

		volume.Set(f, fv)
	}

	if volume.IsCar() {
		set(OPLCarN)
	}

	if volume.IsMod() {
		set(OPLModN)
	}
}

const (
	PitchFrac = 8
	PitchUp   = 31
	PitchDown = 27
)

var win9xFreqs = [...]int{
	0xAB7, 0xB5A, 0xC07, 0xCBE, 0xD80, 0xE4D, 0xF27, 0x100E, 0x1102, 0x1205, 0x1318, 0x143A,
}

var win9xModels = [...]int{
	40, 36, 32, 28, 23, 21, 19, 17,
	15, 14, 13, 12, 11, 10, 9, 8,
	7, 6, 5, 5, 4, 4, 3, 3,
	2, 2, 1, 1, 1, 0, 0, 0,
}

func win9xbend(freq, pitch int) int {

	switch {
	case pitch > 0:
		diff := (pitch * PitchUp) >> PitchFrac
		freq += (diff * freq) >> 15
	case pitch < 0:
		diff := (-pitch * PitchDown) >> PitchFrac
		freq -= (diff * freq) >> 15
	}

	return freq
}
