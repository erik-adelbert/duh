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

// SynthMode is the MIDI dialect announced by the file, as libADLMIDI's m_synthMode.
type SynthMode uint8

const (
	ModeGM SynthMode = 0
	ModeGS SynthMode = 1
	ModeXG SynthMode = 2
)

const (
	vendorRoland       = 0x41
	vendorYamaha       = 0x43
	vendorDUH          = 0x7D
	vendorUniversalNRT = 0x7E
	vendorUniversalRT  = 0x7F

	duhSysExVersion = 0x01
	duhSelectBank   = 0x01

	rolandSend  = 0x12
	rolandModel = 0x42 // GS
	yamahaModel = 0x4C // XG

	masterVolumeDefault = 127
)

// gsDrumChans maps the channel nibble of a GS percussion address to a MIDI channel.
var gsDrumChans = [16]uint8{9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 15}

// SysEx handles a system exclusive message. data is the event payload, which
// follows the F0 status and ends with F7. It reports whether it was understood.
func (s *Synth) SysEx(dev uint32, data []byte) bool {
	if len(data) < 3 || data[len(data)-1] != 0xF7 {
		return false
	}

	vendor, id := data[0], data[1]

	data = data[1 : len(data)-1] // strip vendor ID, keep the rest up to F7

	switch vendor {
	case vendorDUH:
		return s.duhSysEx(data)
	case vendorUniversalNRT, vendorUniversalRT:
		return s.universalSysEx(id, vendor == vendorUniversalRT, data[1:])
	case vendorRoland:
		return s.rolandSysEx(dev, id, data[1:])
	case vendorYamaha:
		return s.yamahaSysEx(id, data[1:])
	}

	return false
}

// duhSysEx handles the private DUH SysEx protocol:
// F0 7D "DUH" version command bank-msb bank-lsb F7.
func (s *Synth) duhSysEx(data []byte) bool {
	if len(data) != 7 ||
		data[0] != 'D' || data[1] != 'U' || data[2] != 'H' ||
		data[3] != duhSysExVersion || data[4] != duhSelectBank ||
		data[5] > 0x7F || data[6] > 0x7F {
		return false
	}

	bank := int(data[5])<<7 | int(data[6])
	return s.SetBank(bank) == nil
}

func (s *Synth) universalSysEx(id byte, realtime bool, data []byte) bool {
	if len(data) < 2 || id != 0x7F && id != s.devID {
		return false
	}

	addr := uint16(data[0]&0x7F)<<8 | uint16(data[1]&0x7F)
	data = data[2:]

	switch {
	case !realtime && addr == 0x0901: // GM System On
		s.smode = ModeGM
		s.ResetState()

		return true
	case !realtime && addr == 0x0902: // GM System Off
		s.smode = ModeXG
		s.ResetState()

		return true
	case realtime && addr == 0x0401 && len(data) == 2: // Master volume
		volume := uint16(data[0]&0x7F) | uint16(data[1]&0x7F)<<7
		s.OPL.SetMasterVolume(int(volume >> 7))

		for ii := range s.mchans {
			s.NoteUpdateAll(uint32(ii), UpdVolume)
		}

		return true
	}

	return false
}

func (s *Synth) rolandSysEx(dev uint32, id byte, data []byte) bool {
	if len(data) < 6 || id != 0x7F && id&0x0F != s.devID {
		return false
	}

	model, mode := data[0]&0x7F, data[1]&0x7F
	sum := data[len(data)-1] & 0x7F
	data = data[2 : len(data)-1]

	var check byte
	for _, b := range data {
		check += b & 0x7F
	}

	if (128-check&127)&127 != sum {
		return false
	}

	addr := uint32(data[0]&0x7F)<<16 | uint32(data[1]&0x7F)<<8 | uint32(data[2]&0x7F)

	var target uint8

	if addr&0xFFF0FF == 0x401015 { // Turn a channel into percussion
		addr = 0x401015
		target = data[1] & 0x0F
	}

	data = data[3:]

	if mode != rolandSend || model != rolandModel {
		return false
	}

	switch addr {
	case 0x00007F, 0x40007F: // System mode set, mode set
		if len(data) != 1 || id&0xF0 != 0x10 {
			break
		}

		s.smode = ModeGS
		s.ResetState()

		return true
	case 0x401015: // Percussion channel
		if len(data) != 1 || id&0xF0 != 0x10 {
			break
		}

		value := data[0] & 0x7F
		s.mchans[dev+uint32(gsDrumChans[target])].isDrum = value == 0x01 || value == 0x02

		return true
	}

	return false
}

func (s *Synth) yamahaSysEx(id byte, data []byte) bool {
	if len(data) < 1 || id != 0x7F && id&0x0F != s.devID {
		return false
	}

	model := data[0] & 0x7F
	data = data[1:]

	if model != yamahaModel || id&0xF0 != 0x10 || len(data) < 3 { // parameter change
		return false
	}

	addr := uint32(data[0]&0x7F)<<16 | uint32(data[1]&0x7F)<<8 | uint32(data[2]&0x7F)
	data = data[3:]

	if addr == 0x00007E && len(data) == 1 { // XG System On
		s.smode = ModeXG
		s.ResetState()

		return true
	}

	return false
}

// ResetState puts every channel back to its power-on state and silences the
// notes, then restores the master volume.
func (s *Synth) ResetState() {
	for ii := range s.mchans {
		ch := &s.mchans[ii]

		ch.Reset()

		ch.lastLRPN, ch.lastMRPN, ch.hasNRPN = 0, 0, false

		if s.smode == ModeGS { // reset custom drum channels
			ch.isDrum = false
		}

		s.NoteUpdateAll(uint32(ii), UpdAll)
		s.NoteUpdateAll(uint32(ii), UpdOff)
	}

	s.OPL.SetMasterVolume(masterVolumeDefault)
}
