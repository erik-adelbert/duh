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

import "math"

// NoteOn starts a MIDI note on a synth channel. midCh is the synth channel
// index, including the offset returned by DeviceSelect.
func (s *Synth) NoteOn(midCh uint32, note, velocity uint8) {
	s.playbackNoteOn(midCh, note, velocity)
}

func (s *Synth) playbackNoteOn(midCh uint32, note, velocity uint8) bool {
	ch := &s.mchans[midCh]

	if s.volume.toon && velocity != 0 {
		if cur, ok := ch.activeNotes[note]; ok {
			cur.vol = velocity
			s.NoteUpdate(midCh, note, UpdVolume, -1)
			return false
		}
	}

	s.NoteOff(midCh, note)
	if velocity == 0 {
		return false
	}

	patch := ch.patch
	if ch.IsDrum(midCh) {
		patch = 0x80 + note
	}

	id := s.GetAdlMetaID(patch, ch)
	meta := s.GetAdlMeta(id)

	tone := int16(note)
	if meta.Tone != 0 {
		switch {
		case ch.IsDrum(midCh):
			tone = int16(meta.Tone)
			if tone >= 128 {
				tone -= 128
			}
		case meta.Tone < 20:
			tone += int16(meta.Tone)
		case meta.Tone < 128:
			tone = int16(meta.Tone)
		default:
			tone -= int16(meta.Tone - 128)
		}
	}

	ids := [2]uint16{meta.ID0, meta.ID1}
	rhythm := meta.Flags & FlagRhythmMask
	if rhythm != 0 || s.adlDrum && s.drums[patch] != 0 {
		ids[1] = ids[0]
	}

	achans := [2]int{-1, -1}

	for voice := range ids {
		if voice == 1 {
			if ids[0] == ids[1] || achans[0] == -1 {
				break
			}
		}

		best := struct {
			i     int
			score int64
		}{-1, int64(math.MinInt64)}

		for i := range s.OPL.chanCount() {
			if voice == 1 && i == achans[0] {
				continue
			}

			if ids[0] == ids[1] || IsPseudo4Op(meta.Flags) {
				want := 0
				if rhythm != 0 {
					want = PercBD + (rhythm-FlagRMBassDrum)/8
				} else if s.adlDrum {
					want = s.drums[patch]

					if s.cmfDrum && midCh >= 11 {
						want = int(3 + midCh - 11)
					}
				}

				if s.OPL.quads[i] != int8(want) {
					continue
				}
			} else {
				switch {
				case voice == 0 && s.OPL.quads[i] != 1:
					continue
				case voice == 1 && i != achans[0]+3:
					continue
				}
			}

			score := s.ChannelScore(uint32(i), uint32(ids[voice]), midCh)
			if score > best.score {
				best.score, best.i = score, i
			}
		}

		if best.i < 0 {
			continue
		}

		s.PrepareForNewNote(best.i, int(ids[voice]))
		achans[voice] = best.i
	}

	if achans[0] < 0 && achans[1] < 0 {
		return false
	}

	if ch.activeNotes == nil {
		ch.activeNotes = make(map[uint8]*noteInfo, 8)
	}

	if ch.isSoft {
		velocity = uint8(float32(velocity) * 0.8)
	}

	info := &noteInfo{
		vol:       velocity,
		tone:      tone,
		pressure:  ch.noteAftertouch[note&0x7F],
		glide:     float64(tone),
		glideRate: math.Inf(1),
		Midi:      patch,
		Meta:      id,
		phys:      make(map[int]uint16),
	}

	if ch.enabled && !math.IsInf(ch.rate, 1) &&
		!ch.IsDrum(midCh) && ch.src >= 0 {
		info.glide = float64(ch.src)
		info.glideRate = ch.rate
	}
	ch.src = int8(note)

	for voice, adlCh := range achans {
		if adlCh >= 0 {
			info.phys[adlCh] = ids[voice]
		}
	}

	ch.activeNotes[note] = info
	s.NoteUpdate(midCh, note, UpdAll|UpdPatch, -1)
	return true
}

// NoteAftertouch applies polyphonic pressure to a note and remembers it for
// the next note-on of the same key. midCh is the decoder channel index.
func (s *Synth) NoteAftertouch(midCh uint32, note, pressure uint8) {
	ch := &s.mchans[midCh]
	if info, ok := ch.activeNotes[note]; ok {
		info.pressure = pressure
	}

	k := note & 0x7F
	if ch.noteAftertouch[k] == pressure {
		return
	}

	ch.noteAftertouch[k] = pressure
	ch.noteTouchEnable = pressure != 0
	for _, v := range ch.noteAftertouch {
		ch.noteTouchEnable = ch.noteTouchEnable || v != 0
	}
}

// Controller applies a channel-mode or continuous-controller message. midCh is
// the decoder channel index, and ctrl and value are MIDI data bytes.
func (s *Synth) Controller(midCh uint32, ctrl, value uint8) {
	ch := &s.mchans[midCh]

	switch ctrl {
	case 0: // Bank MSB
		ch.bank.MSB = value
		ch.hasBank = true
		if s.smode != ModeGS {
			ch.isDrum = ch.IsXGDrum()
		}
	case 1: // Modulation wheel / vibrato
		ch.vibrato.lvl = value
	case 5: // Portamento MSB
		ch.portamento.lvl = (ch.portamento.lvl & 0x7F) | (uint16(value) << 7)
		s.UpdatePortamento(midCh)
	case 6: // Data entry MSB (RPN)
		s.SetRPN(midCh, uint32(value), true)
	case 7: // Channel volume
		ch.vol = value
		s.NoteUpdateAll(midCh, UpdVolume)
	case 10: // Pan
		ch.panning = value
		s.NoteUpdateAll(midCh, UpdPan)
	case 11: // Expression
		ch.expr = value
		s.NoteUpdateAll(midCh, UpdVolume)
	case 32: // Bank LSB
		ch.bank.LSB = value
		if s.smode != ModeGS {
			ch.isDrum = ch.IsXGDrum()
		}
		ch.hasBank = true
	case 37: // Portamento LSB
		ch.portamento.lvl = (ch.portamento.lvl & 0x3F80) | uint16(value)
		s.UpdatePortamento(midCh)
	case 38: // Data entry LSB (RPN)
		s.SetRPN(midCh, uint32(value), false)
	case 64: // Sustain pedal
		ch.sustain = value
		if value == 0 {
			s.KillSustainingNotes(int(midCh), -1, holdPedal)
		}
	case 66: // Sostenuto pedal
		if value >= 64 {
			s.MarkSostenutoNotes(int(midCh))
		} else {
			s.KillSustainingNotes(int(midCh), -1, holdSostenuto)
		}
	case 67: // Soft pedal
		ch.isSoft = value >= 64
	case 65: // Portamento enable/disable
		ch.enabled = value >= 64
		s.UpdatePortamento(midCh)
	case 74: // Brightness
		ch.bright = value
		s.NoteUpdateAll(midCh, UpdVolume)
	case 91, 92, 93, 94, 95: // Effect depths - not supported
	case 98: // NRPN LSB
		ch.lastLRPN = value
		ch.hasNRPN = true
	case 99: // NRPN MSB
		ch.lastMRPN = value
		ch.hasNRPN = true
	case 100: // RPN LSB
		ch.lastLRPN = value
		ch.hasNRPN = false
	case 101: // RPN MSB
		ch.lastMRPN = value
		ch.hasNRPN = false
	case 103: // Channel mode message (CMF Drum)
		s.cmfDrum = value != 0
	case 113: // Related to pitch-bender
	case 121: // Reset all controllers
		ch.ResetControllers()
		if s.volume.toon {
			ch.vol = ch.vol0
		}
		s.UpdatePortamento(midCh)
		s.NoteUpdateAll(midCh, UpdPan|UpdVolume|UpdPitch)
		s.KillSustainingNotes(int(midCh), -1, holdAny)
	case 120: // All sounds off
		for note := range ch.activeNotes {
			s.NoteUpdate(midCh, note, UpdOff|UpdMute, -1)
		}
	case 123: // All notes off
		s.NoteUpdateAll(midCh, UpdOff)
	}
}

// ProgramChange selects the MIDI instrument for a channel. midCh is the
// decoder channel index.
func (s *Synth) ProgramChange(midCh uint32, program uint8) {
	s.mchans[midCh].patch = program
}

// ChannelPressure sets channel-wide aftertouch. midCh is the decoder channel
// index.
func (s *Synth) ChannelPressure(midCh uint32, pressure uint8) {
	s.mchans[midCh].aftertouch = pressure
}

// PitchBend applies a 14-bit MIDI pitch-bend value (0..16383). midCh is the
// decoder channel index.
func (s *Synth) PitchBend(midCh uint32, value uint16) {
	ch := &s.mchans[midCh]
	ch.bend = float64(value) - 8192
	s.NoteUpdateAll(midCh, UpdPitch)
}

func (s *Sequencer) handleMIDIEvent(tr *trackInfo, status uint8, midCh uint32) {
	switch status >> 4 {
	case 0x08, 0x09:
		note, _ := tr.ReadByte()
		velocity, _ := tr.ReadByte()
		if status>>4 == 0x08 {
			s.playbackNoteOff(midCh, note, velocity)
			return
		}
		if s.playbackNoteOn(midCh, note, velocity) {
			s.pos.cur.isPlaying = true
		}
	case 0x0A:
		note, _ := tr.ReadByte()
		pressure, _ := tr.ReadByte()
		s.NoteAftertouch(midCh, note, pressure)
	case 0x0B:
		ctrl, _ := tr.ReadByte()
		value, _ := tr.ReadByte()
		s.Controller(midCh, ctrl, value)
	case 0x0C:
		program, _ := tr.ReadByte()
		s.ProgramChange(midCh, program)
	case 0x0D:
		pressure, _ := tr.ReadByte()
		s.ChannelPressure(midCh, pressure)
	case 0x0E:
		lsb, _ := tr.ReadByte()
		msb, _ := tr.ReadByte()
		s.PitchBend(midCh, uint16(lsb)|uint16(msb)<<7)
	}
}
