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

package model

import "math"

type Hmi struct{}

func (g *Hmi) Freq(tone float64, offset *uint32) uint16 {
	*offset = 0

	note := int(math.Trunc(tone))
	half := tone - math.Trunc(tone)

	if half > 0.5 {
		note++
		half -= 1.0
	}

	var off int

	for note < 12 {
		off--
		note += 12
	}

	for note > 114 {
		off++
		note -= 12
	}

	pitch := int(half*64) + 64

	f0 := hmiFreqs[note-12]
	if pitch != 64 {
		f0 = hmibend(note, pitch)
	}

	freq := f0 & 0x3FF

	oct := max(((f0>>10)&0x07)+off, 0)

	for oct > 7 {
		*offset++
		oct--
	}

	return uint16(oct<<10 | freq)
}

// hmiLevel returns the HMI attenuation table entry for the given volume terms.
func hmiLevel(chVol, chExpr, mv, vel int) int {
	v := chVol * chExpr * mv / 16129
	v = ((v * 128 / 127) * vel) >> 7

	return hmiModels[min(v, 127)>>1]
}

// Volume is the HMI SOS model.
func (g *Hmi) Volume(volume *OPLModel) {
	v := hmiLevel(
		volume.Get(OPLChanVolume),
		volume.Get(OPLChanExpression),
		volume.Get(OPLMasterVolume),
		volume.Get(OPLVelocity),
	)

	set := func(f OPLField) {
		out := (64 - v) << 1
		out *= 64 - volume.Get(f)

		volume.Set(f, (8192-out)>>7)
	}

	if volume.IsMod() {
		set(OPLModN)
	}

	if volume.IsCar() {
		set(OPLCarN)
	}
}

// HmiOld is the older HMI SOS model: the carrier alone follows the volume
// and the modulator of FM voices is derived from the channel volume.
type HmiOld struct{ Hmi }

func (g *HmiOld) Volume(volume *OPLModel) {
	vol := volume.Get(OPLChanVolume)
	exp := volume.Get(OPLChanExpression)
	vel := volume.Get(OPLVelocity)

	v := hmiLevel(vol, exp, volume.Get(OPLMasterVolume), vel)
	car := 64 - volume.Get(OPLCarN)

	if volume.Get(OPLFbconn) == 0 && !volume.IsDrum() {
		out := (64 - hmiLevel(vol, exp, 64, vel)) << 1

		volume.Set(OPLModN, (8192-out*car)>>7)
	}

	out := (64 - v) << 1
	if volume.IsDrum() {
		// TODO: verify the correctness of this in libADLMIDI too.
		out = (64 - hmiModels[vel>>1]) << 1
	}

	volume.Set(OPLCarN, (8192-out*car)>>7)
}

var bends = [...]int{
	0x144, 0x132, 0x121, 0x110, 0x101, 0xf8, 0xe5, 0xd8, 0xcc, 0xc1, 0xb6, 0xac,
}

func hmibend(note, pitch int) int {
	const span = 1

	note -= 12

	freq := uint32(hmiFreqs[note])
	oct := freq & 0x1c00
	fm := freq & 0x3ff
	flen := len(hmiFreqs)

	// libADLMIDI does this in unsigned arithmetic: a "negative" difference
	// wraps around and is caught by the > 719 test.
	if pitch < 64 {
		bf := uint32((63-pitch)*1000) >> 6

		nf := freq - uint32(hmiFreqs[clamp(note-span, 0, flen)])
		if nf > 719 {
			nf = (fm - uint32(bends[span-1])) & 0x3ff
		}

		nf = (nf * bf) / 1000
		freq -= nf
	} else {
		bf := uint32((pitch-64)*1000) >> 6

		nf := uint32(hmiFreqs[clamp(note+span, 0, flen)]) - freq
		if nf > 719 {
			fm = uint32(bends[clamp(11-note%12, 0, len(bends))])
			freq = (oct + 1024) | fm
			nf = uint32(hmiFreqs[clamp(note+span, 0, flen)]) - freq
		}

		nf = (nf * bf) / 1000
		freq += nf
	}

	return int(freq)
}

func clamp(v int, min, max int) int {
	if v < min {
		return min
	}

	if v >= max {
		return max - 1
	}

	return v
}

var hmiFreqs = [...]int{
	0x0157, 0x016B, 0x0181, 0x0198, 0x01B0, 0x01CA, 0x01E5, 0x0202, 0x0220, 0x0241, 0x0263, 0x0287,
	0x0557, 0x056B, 0x0581, 0x0598, 0x05B0, 0x05CA, 0x05E5, 0x0602, 0x0620, 0x0641, 0x0663, 0x0687,
	0x0957, 0x096B, 0x0981, 0x0998, 0x09B0, 0x09CA, 0x09E5, 0x0A02, 0x0A20, 0x0A41, 0x0A63, 0x0A87,
	0x0D57, 0x0D6B, 0x0D81, 0x0D98, 0x0DB0, 0x0DCA, 0x0DE5, 0x0E02, 0x0E20, 0x0E41, 0x0E63, 0x0E87,
	0x1157, 0x116B, 0x1181, 0x1198, 0x11B0, 0x11CA, 0x11E5, 0x1202, 0x1220, 0x1241, 0x1263, 0x1287,
	0x1557, 0x156B, 0x1581, 0x1598, 0x15B0, 0x15CA, 0x15E5, 0x1602, 0x1620, 0x1641, 0x1663, 0x1687,
	0x1957, 0x196B, 0x1981, 0x1998, 0x19B0, 0x19CA, 0x19E5, 0x1A02, 0x1A20, 0x1A41, 0x1A63, 0x1A87,
	0x1D57, 0x1D6B, 0x1D81, 0x1D98, 0x1DB0, 0x1DCA, 0x1DE5, 0x1E02, 0x1E20, 0x1E41, 0x1E63, 0x1E87,
	0x1EAE, 0x1EB7, 0x1F02, 0x1F30, 0x1F60, 0x1F94, 0x1FCA,
}

var hmiModels = [...]int{
	0x3F, 0x3A, 0x35, 0x30, 0x2C, 0x29, 0x25, 0x24,
	0x23, 0x22, 0x21, 0x20, 0x1F, 0x1E, 0x1D, 0x1C,
	0x1B, 0x1A, 0x19, 0x18, 0x17, 0x16, 0x15, 0x14,
	0x13, 0x12, 0x11, 0x10, 0x0F, 0x0E, 0x0E, 0x0D,
	0x0D, 0x0C, 0x0C, 0x0B, 0x0B, 0x0A, 0x0A, 0x09,
	0x09, 0x08, 0x08, 0x07, 0x07, 0x06, 0x06, 0x06,
	0x05, 0x05, 0x05, 0x04, 0x04, 0x04, 0x04, 0x03,
	0x03, 0x03, 0x02, 0x02, 0x02, 0x01, 0x01, 0x00,
}

// ImfCreator combines the HMI frequency model with the fixed DMX volume model.
type ImfCreator struct {
	Hmi
	DmxFixed
}

func (g *ImfCreator) Freq(tone float64, offset *uint32) uint16 { return g.Hmi.Freq(tone, offset) }

func (g *ImfCreator) Volume(volume *OPLModel) { g.DmxFixed.Volume(volume) }
