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

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Synth maintains MIDI channel and OPL voice state independently of file playback.
type Synth struct {
	devices map[string]uint32
	devID   uint8

	mchans []midiChan
	achans []adlChan

	OPL  *OPL3
	bank uint32

	vmode  int
	volume struct{ log, toon bool }

	smode SynthMode

	adlDrum bool
	cmfDrum bool

	drums [256]int

	dyno struct {
		ins   []adlData
		metas []adlMeta
	}

	arpeggio uint32
}

const NOID = ^uint16(0)

// adlChan represents an AdLib channel
type adlChan struct {
	users     map[location]locData
	KOFFDecay time.Duration
}

// location identifies a note on a MIDI channel
type location struct {
	mchan uint16
	note  uint8
}

// Hold tells which pedals keep a released note sounding.
type Hold uint8

const (
	HoldNone      Hold = 0
	HoldPedal     Hold = 1 // sustain pedal
	HoldSostenuto Hold = 2 // sostenuto pedal
	HoldAny       Hold = HoldPedal | HoldSostenuto
)

// locData holds data for a location
type locData struct {
	sustained Hold
	ins       uint16
	KONDecay  time.Duration
	vibDelay  time.Duration
}

// NewSynth creates an OPL synth with initialized voice state.
// Call DeviceSelect to create MIDI channels before sending events.
func NewSynth(cardCount, quadOpsCount, flags int) *Synth {
	return newSynth(cardCount, quadOpsCount, flags, uint32(BankSelect))
}

func newSynth(cardCount, quadOpsCount, flags int, bank uint32) *Synth {
	s := &Synth{
		devices: make(map[string]uint32),
		OPL:     newOPL3(cardCount, quadOpsCount, flags),
		bank:    bank,
		smode:   ModeXG,
		adlDrum: has(flags, ModeADLPercussion),
	}
	s.SetVolumeModel(0)
	s.applyBankMode()
	s.initADLChannels()
	return s
}

func (s *Synth) playbackNoteOff(channel uint32, note, velocity uint8) {
	if s.volume.toon && velocity != 0 {
		if cur, ok := s.mchans[channel].activeNotes[note]; ok {
			cur.vol = velocity
			s.NoteUpdate(channel, note, UpdVolume, -1)
			return
		}
	}
	s.NoteOff(channel, note)
}

func (s *Synth) prepareFilePlayback() error { return s.initADLChannels() }

func (s *Synth) setFileFormat(format Format) { s.OPL.cmf = format == FormatCMF }

func (s *Synth) addCMFInstrument(data adlData) {
	id := uint16(len(s.dyno.ins)) | DynamicInstrumentTag
	s.dyno.metas = append(s.dyno.metas, adlMeta{ID0: id, ID1: id, KON: 1000, KOFF: 500})
	s.dyno.ins = append(s.dyno.ins, data)
}

func (s *Synth) useCMFBank() {
	s.bank = ^uint32(0)
	s.volume.log = true
}

func (s *Synth) useRSXXDefaults() {
	s.volume.log = true
	s.volume.toon = true
	s.SetDefaults()
}

func (s *Synth) writeOPL(reg, value int) { s.OPL.poke(0, reg, value) }

// SetVolumeModel selects the volume model: 0 picks the one declared by the
// current bank, 1..14 select a model like libADLMIDI's -vm option
// (1 generic, 2 native, 3 DMX, ... 14 O'Connell).
func (s *Synth) SetVolumeModel(id int) {
	s.vmode = id

	if id == 0 {
		id = bankVolumeModel(s.bank)
	}

	s.OPL.setVolumeModel(newVolumeModel(id - 1))
}

// SetBank selects the ADL bank family used for future note lookups.
func (s *Synth) SetBank(bank int) error {
	if bank < 0 || bank >= len(banks) {
		return fmt.Errorf("ADL bank %d is out of range (0..%d)", bank, len(banks)-1)
	}

	s.bank = uint32(bank)
	s.SetVolumeModel(s.vmode)
	s.applyBankMode()

	vol, bend := s.defaults()
	for ii := range s.mchans {
		s.mchans[ii].vol0 = vol
		s.mchans[ii].pitch0 = bend
	}

	return nil
}

func (s *Synth) applyBankMode() {
	var setup uint16
	rhythm := has(s.OPL.Flags, ModeADLPercussion)

	bank := int(s.bank)
	if bank >= 0 && bank < len(banks) {
		setup = banks[bank].Setup

		for _, mb := range banks[bank].Drums {
			for _, id := range mb.Programs {
				if int(id) < len(adlMetas) && adlMetas[id].Flags&FlagRhythmMask != 0 {
					rhythm = true
				}
			}
		}
	}

	if len(s.OPL.bds) > 0 && (s.OPL.bds[0]&0x20 != 0) != rhythm {
		for card := range s.OPL.Cards {
			for _, ii := range []int{6, 7, 8, 18, 19, 20, 21, 22} {
				c := card*23 + ii
				if c < len(s.achans) {
					for loc := range s.achans[c].users {
						if int(loc.mchan) < len(s.mchans) {
							s.NoteUpdate(uint32(loc.mchan), loc.note, UpdOff|UpdMute, -1)
						}
					}
					clear(s.achans[c].users)
					s.achans[c].KOFFDecay = 0
				}
				s.OPL.noteOff(c)
				s.OPL.mute(c)
			}
		}
	}

	s.OPL.setBankMode(setup, rhythm)
}

// defaults returns the default channel volume and pitch-bend sensitivity:
// MT-32 banks and RSXX files start at full volume with a 12 semitones range,
// like libADLMIDI's resetMIDIDefaults.
func (s *Synth) defaults() (vol, bend uint8) {
	if s.volume.toon || s.mt32() {
		return 127, 12
	}

	return 100, 2
}

// mt32 reports whether the current bank declares MT-32 defaults.
func (s *Synth) mt32() bool {
	bank := int(s.bank)

	return bank >= 0 && bank < len(banks) && banks[bank].Setup&(1<<10) != 0
}

// SetDefaults applies the default volume and bend sensitivity to every channel.
func (s *Synth) SetDefaults() {
	vol, bend := s.defaults()

	for ii := range s.mchans {
		ch := &s.mchans[ii]

		ch.vol0, ch.pitch0 = vol, bend
		ch.vol = vol
		ch.pitch.MSB = int(bend)
	}
}

func (s *Synth) initADLChannels() error {
	s.achans = make([]adlChan, s.OPL.chanCount())
	for i := range s.achans {
		s.achans[i].users = make(map[location]locData)
	}

	return nil
}

// Advance updates synth effects and voice timers by elapsed time.
func (s *Synth) Advance(dt time.Duration) {
	for c := range s.achans {
		s.achans[c].Advance(dt)
	}
	s.UpdateGlide(dt)
	s.UpdateVibrato(dt)
	s.UpdateArpeggio(dt)
}

const minDecay = -0x1FFFFFFF * time.Millisecond

// Advance advances the channel's per-user decay/vibrato timers by dt.
func (ac *adlChan) Advance(dt time.Duration) {
	if len(ac.users) == 0 {
		ac.KOFFDecay = max(ac.KOFFDecay-dt, minDecay)
		return
	}

	ac.KOFFDecay = 0
	for k, v := range ac.users {
		v.KONDecay = max(v.KONDecay-dt, minDecay)
		v.vibDelay += dt
		ac.users[k] = v
	}
}

// Update flags
const (
	UpdPatch  = 0x1
	UpdPan    = 0x2
	UpdVolume = 0x4
	UpdPitch  = 0x8
	UpdAll    = UpdPan + UpdVolume + UpdPitch
	UpdOff    = 0x20
	UpdMute   = 0x40
)

// NoteUpdate updates a note
func (s *Synth) NoteUpdate(midCh uint32, note uint8, propsMask uint32, ADLChan int) {
	info, ok := s.mchans[midCh].activeNotes[note]

	if !ok {
		return
	}

	vol := info.vol

	meta := info.instrument.Meta
	ains := s.GetAdlMeta(meta)
	if has(propsMask, UpdVolume) {
		vol = uint8(max(0, min(127, int(vol)+int(ains.VelocityOffset))))
	}

	loc := location{mchan: uint16(midCh), note: note}

	for ch, ins := range info.phys {
		if ADLChan >= 0 && ch != ADLChan {
			continue
		}

		if has(propsMask, UpdPatch) {
			s.OPL.patch(ch, ins)

			d := s.achans[ch].users[loc]

			d.sustained = HoldNone
			d.vibDelay = 0
			d.KONDecay = ains.KON
			d.ins = ins

			s.achans[ch].users[loc] = d
		}
	}

	for ch, ins := range info.phys {
		if ADLChan >= 0 && ch != ADLChan {
			continue
		}

		if has(propsMask, UpdOff) {
			if has(propsMask, UpdMute) {
				delete(s.achans[ch].users, loc)
				if len(s.achans[ch].users) == 0 {
					s.OPL.noteOff(ch)
					s.OPL.mute(ch)
					s.achans[ch].KOFFDecay = 0
				}
			} else if s.mchans[midCh].sustain == 0 {
				// A sostenuto-held note stays until the pedal is released
				if s.achans[ch].users[loc].sustained&HoldSostenuto == 0 {
					delete(s.achans[ch].users, loc)

					if len(s.achans[ch].users) == 0 {
						s.OPL.noteOff(ch)

						s.achans[ch].KOFFDecay = ains.KOFF
					}
				}
			} else {
				d := s.achans[ch].users[loc]
				d.sustained |= HoldPedal

				s.achans[ch].users[loc] = d
			}

			delete(info.phys, ch)
			continue
		}

		if has(propsMask, UpdPan) {
			s.OPL.pan(ch, int(s.mchans[midCh].panning))
		}

		if has(propsMask, UpdVolume) {
			c := &s.mchans[midCh]

			// Brightness acts like a post high-pass filter: it only
			// attenuates by half its range, like libADLMIDI's default.
			bright := int(c.bright)
			if bright >= 64 {
				bright = 127
			} else {
				bright *= 2
			}

			s.OPL.touch(ch, int(vol), int(c.vol), int(c.expr), bright, c.IsDrum(midCh))
		}

		if has(propsMask, UpdPitch) {
			d := s.achans[ch].users[loc]

			if d.sustained == HoldNone {
				pitch := s.mchans[midCh].pitch.Value() + float64(s.GetAdlIns(ins).Finetune)

				if IsPseudo4Op(ains.Flags) && ins == ains.ID1 {
					pitch += ains.SecondFinetune
				}

				vib := s.mchans[midCh].vibrato
				level := max(vib.lvl, s.mchans[midCh].aftertouch, info.pressure)

				if level != 0 && d.vibDelay >= vib.delay {
					pitch += float64(level) * vib.depth *
						math.Sin(vib.pos)
				}

				s.OPL.noteOn(ch, info.glide+pitch)
			}
		}
	}

	if len(info.phys) == 0 {
		delete(s.mchans[midCh].activeNotes, note)
	}
}

// ChannelScore determines how good a candidate channel is
func (m *Synth) ChannelScore(c0, ins, midCh uint32) int64 {
	s := -m.achans[c0].KOFFDecay.Milliseconds()

	for loc, data := range m.achans[c0].users {
		s -= 4000

		if data.sustained == HoldNone {
			s -= data.KONDecay.Milliseconds()
		} else {
			s -= data.KONDecay.Milliseconds() / 2
		}

		if info, ok := m.mchans[loc.mchan].activeNotes[loc.note]; ok {
			if data.ins == uint16(ins) {
				s += 300
			}

			s += 50 * int64(info.instrument.Midi/128)

			var nevac int64

			for c1 := range m.OPL.chanCount() {
				if c1 == int(c0) {
					continue
				}

				if m.OPL.quads[c1] != m.OPL.quads[c0] {
					continue
				}

				for _, mData := range m.achans[c1].users {
					if mData.sustained != HoldNone || mData.vibDelay >= 200 || mData.ins != data.ins {
						continue
					}

					nevac++
				}
			}

			s += nevac * 4
		}
	}

	return s
}

// PrepareForNewNote prepares a channel for a new note
func (s *Synth) PrepareForNewNote(c, ins int) {
	if len(s.achans[c].users) == 0 {
		return
	}

	for loc, data := range s.achans[c].users {
		if data.sustained == HoldNone {
			if (data.vibDelay < 70 || data.KONDecay > 20000) && data.ins == uint16(ins) {
				continue // Do arpeggio
			}
			s.MoveOrKill(uint32(c), loc, data)
		}
	}

	// Kill all sustained notes on this channel.
	// Don't keep them for arpeggio, because arpeggio requires
	// an intact "activenotes" record. This is a design flaw.
	s.KillSustainingNotes(-1, c, HoldAny)

	if len(s.achans[c].users) == 0 {
		s.OPL.noteOff(c)
	}
}

// MoveOrKill kills or evacuates a note
func (s *Synth) MoveOrKill(fromChannel uint32, loc location, data locData) {
	for c := range s.OPL.chanCount() {
		if uint32(c) == fromChannel {
			continue
		}

		if s.OPL.quads[c] != s.OPL.quads[fromChannel] {
			continue
		}

		for _, locdat := range s.achans[c].users {
			if locdat.vibDelay >= 200 && locdat.KONDecay < 10000 {
				continue
			}

			if locdat.ins != data.ins {
				continue
			}

			// Move the note to another channel if possible
			if info, ok := s.mchans[loc.mchan].activeNotes[loc.note]; ok {
				delete(info.phys, int(fromChannel))

				info.phys[int(c)] = data.ins

				s.achans[c].users[loc] = data
				delete(s.achans[fromChannel].users, loc)

				return
			}
		}
	}

	// Kill
	if _, ok := s.mchans[loc.mchan].activeNotes[loc.note]; ok {
		s.NoteUpdate(uint32(loc.mchan), loc.note, UpdOff, int(fromChannel))
	}
}

// KillSustainingNotes releases the given kind of hold on notes, and kills
// those no longer held by any pedal.
func (s *Synth) KillSustainingNotes(midCh, baseCh int, kind Hold) {
	first, last := 0, s.OPL.chanCount()

	if baseCh >= 0 {
		first = baseCh
		last = first + 1
	}

	for i := first; i < last; i++ {
		if len(s.achans[i].users) == 0 {
			continue
		}

		for loc, data := range s.achans[i].users {
			if (midCh < 0 || int(loc.mchan) == midCh) && data.sustained&kind != 0 {
				data.sustained &^= kind

				if data.sustained == HoldNone {
					delete(s.achans[i].users, loc)
				} else {
					s.achans[i].users[loc] = data
				}
			}
		}

		if len(s.achans[i].users) == 0 {
			s.OPL.noteOff(i)
		}
	}
}

// MarkSostenutoNotes holds the notes currently sounding on a MIDI channel.
func (s *Synth) MarkSostenutoNotes(midCh int) {
	for i := range s.OPL.chanCount() {
		for loc, data := range s.achans[i].users {
			if int(loc.mchan) == midCh && data.sustained == HoldNone {
				data.sustained |= HoldSostenuto
				s.achans[i].users[loc] = data
			}
		}
	}
}

// SetRPN sets an RPN parameter
func (s *Synth) SetRPN(midCh, value uint32, msb bool) {
	nrpn := s.mchans[midCh].hasNRPN
	addr := uint32(s.mchans[midCh].lastMRPN)*0x100 + uint32(s.mchans[midCh].lastLRPN)
	key := addr

	if nrpn {
		key += 0x10000
	}

	if msb {
		key += 0x20000
	}

	ch := &s.mchans[midCh]

	switch key {
	case 0x0000 + 0*0x10000 + 0*0x20000: // Pitch-bender sensitivity
		ch.pitch.LSB = int(value)
	case 0x0000 + 0*0x10000 + 1*0x20000: // Pitch-bender sensitivity
		ch.pitch.MSB = int(value)
	case 0x0108 + 1*0x10000 + 1*0x20000: // Vibrato speed
		if value == 64 {
			ch.vibrato.vel = 1.0
		} else if value < 100 {
			v := value
			if v == 0 {
				v = 1
			}
			ch.vibrato.vel = 1.0 / (1.6e-2 * float64(v))
		} else {
			ch.vibrato.vel = 1.0 / (0.051153846*float64(value) - 3.4965385)
		}
		ch.vibrato.vel *= 2 * math.Pi * 5.0
	case 0x0109 + 1*0x10000 + 1*0x20000: // Vibrato depth
		ch.vibrato.depth = (float64(value) - 64) * 0.15 * 0.01
	case 0x010A + 1*0x10000 + 1*0x20000: // Vibrato delay
		if value != 0 {
			ch.vibrato.delay = time.Duration(0.2092 * math.Exp(0.0795*float64(value)) * float64(time.Second))
		} else {
			ch.vibrato.delay = 0
		}
	}
}

// UpdatePortamento updates portamento
func (s *Synth) UpdatePortamento(midCh uint32) {
	p := &s.mchans[midCh].portamento

	p.rate = math.Inf(1)

	if p.enabled && p.lvl > 0 {
		p.rate = 350.0 * math.Pow(2, -0.062*(1.0/128)*float64(p.lvl))
	}
}

// UpdateGlide slides the gliding notes of all channels towards their tone.
func (s *Synth) UpdateGlide(amount time.Duration) {
	sec := amount.Seconds()

	for a := range s.mchans {
		for note, info := range s.mchans[a].activeNotes {
			if math.IsInf(info.glideRate, 1) {
				continue
			}

			final, prev := float64(info.tone), info.glide

			next := prev - sec*info.glideRate
			if prev < final {
				next = min(prev+sec*info.glideRate, final)
			} else {
				next = max(next, final)
			}

			if int64(next*1e6) != int64(prev*1e6) {
				info.glide = next
				s.NoteUpdate(uint32(a), note, UpdPitch, -1)
			}
		}
	}
}

// NoteUpdateAll updates all notes on a channel
func (s *Synth) NoteUpdateAll(midCh uint32, propsMask uint32) {
	for note := range s.mchans[midCh].activeNotes {
		s.NoteUpdate(midCh, note, propsMask, -1)
	}
}

// NoteOff turns off a note
func (s *Synth) NoteOff(midCh uint32, note uint8) {
	if _, ok := s.mchans[midCh].activeNotes[note]; ok {
		s.NoteUpdate(midCh, note, UpdOff, -1)
	}
}

// UpdateVibrato updates vibrato for all channels
func (s *Synth) UpdateVibrato(amount time.Duration) {
	sec := amount.Seconds()

	for a := range s.mchans {
		ch := &s.mchans[a]

		if ch.HasVibrato() && len(ch.activeNotes) > 0 {
			s.NoteUpdateAll(uint32(a), UpdPitch)
			ch.vibrato.pos += sec * ch.vibrato.vel
		} else {
			ch.vibrato.pos = 0.0
		}
	}
}

// UpdateArpeggio updates arpeggio for all channels
func (s *Synth) UpdateArpeggio(_ time.Duration) {
	// Arpeggio logic
	for c := range s.OPL.chanCount() {
		nuser := len(s.achans[c].users)
		if nuser > 1 {
			// Sort users for deterministic iteration
			var locs []location

			for loc := range s.achans[c].users {
				locs = append(locs, loc)
			}

			sort.Slice(locs, func(i, j int) bool {
				if locs[i].mchan != locs[j].mchan {
					return locs[i].mchan < locs[j].mchan
				}

				return locs[i].note < locs[j].note
			})

			// Simple round-robin arpeggio
			s.arpeggio++

			invrate := 3
			switch {
			case nuser >= 4:
				invrate = 1
			case nuser >= 3:
				invrate = 2
			}

			idx := (int(s.arpeggio) / invrate) % nuser
			loc := locs[idx]
			data := s.achans[c].users[loc]

			if data.sustained == HoldNone {
				if data.KONDecay <= 0 {
					s.NoteUpdate(uint32(loc.mchan), loc.note, UpdOff, c)
				} else {
					s.NoteUpdate(uint32(loc.mchan), loc.note, UpdPitch|UpdVolume|UpdPan, c)
				}
			}
		}
	}
}

// DeviceSelect chooses a device for a track
func (s *Synth) DeviceSelect(name string) uint32 {
	// fmt.Fprintf(os.Stderr, "choosing device '%s'\n", name)

	if dev, ok := s.devices[name]; ok {
		return dev
	}

	n := uint32(len(s.devices)) * 16
	s.devices[name] = n

	vol, bend := s.defaults()

	// Resize channels
	for len(s.mchans) < int(n)+16 {
		s.mchans = append(s.mchans, midiChan{
			vol:     vol,
			vol0:    vol,
			pitch0:  bend,
			expr:    127,
			bright:  127,
			panning: 64,
			pitch:   pitch{MSB: int(bend)},

			src:  -1,
			rate: math.Inf(1),

			vel:   2 * 3.141592653 * 5.0,
			depth: 0.5 / 127,

			activeNotes: make(map[uint8]*noteInfo),
		})
	}
	return n
}

// Dynamic instrument IDs are ordinary indices with bit 15 set.
// The low bits are the index into m.dyn.ins / m.dyn.metas.
const (
	DynamicIndexMask         uint16 = 0x7FFF
	DynamicInstrumentTag     uint16 = 0x8000
	DynamicMetaInstrumentTag uint32 = 0x4000000
)

// isDynamic reports whether id refers to a dynamically-loaded instrument.
func isDynamic(id uint16) bool { return has(id, DynamicInstrumentTag) }

// dyn returns the index into m.dyn.* for a dynamic id.
func dyn(id uint16) int { return int(id & DynamicIndexMask) }

// mkdyn encodes an index as a dynamic instrument id.
func mkdyn(idx int) uint16 { return uint16(idx) | DynamicInstrumentTag }

// GetAdlMetaID maps an instrument index (as stored in a track) to the
// meta-instrument id used by GetAdlMeta.
//
// When a dynamic bank is active, the result is a dynamic id; otherwise
// it is the meta-instrument ID selected from the derived BankDB tables.
func (s *Synth) GetAdlMetaID(ins uint8, channel *midiChan) uint16 {
	if s.bank == ^uint32(0) {
		return mkdyn(int(ins))
	}

	bank := int(s.bank)
	if len(banks) == 0 {
		return 0
	}
	if bank < 0 || bank >= len(banks) {
		bank = 0
	}

	patch := int(ins)
	midiBanks := banks[bank].Melos
	if patch >= 128 {
		patch -= 128
		midiBanks = banks[bank].Drums
	}
	if patch >= 128 || len(midiBanks) == 0 {
		return 0
	}

	selected := midiBanks[0]
	if channel != nil && channel.hasBank {
		for _, midiBank := range midiBanks {
			if midiBank.bank == channel.bank {
				selected = midiBank
				break
			}
		}
	}
	return selected.Programs[patch]
}

// GetAdlMeta returns the meta-instrument for a meta id produced by
// GetAdlMetaID. Dynamic ids are resolved against the dynamic bank;
// everything else against the built-in ADLMetas table.
func (s *Synth) GetAdlMeta(metaID uint16) adlMeta {
	if isDynamic(metaID) {
		return s.dynamicMeta(dyn(metaID))
	}
	return s.staticMeta(int(metaID))
}

func (s *Synth) dynamicMeta(idx int) adlMeta {
	if idx < 0 || idx >= len(s.dyno.metas) {
		return adlMeta{}
	}
	return s.dyno.metas[idx]
}

func (s *Synth) staticMeta(idx int) adlMeta {
	if idx < 0 || idx >= len(adlMetas) {
		return adlMeta{}
	}
	return adlMetas[idx]
}

// GetAdlIns returns the AdLib instrument data for an instrument id.
// Dynamic ids are resolved against the dynamic bank; everything else
// against the built-in ADLIns table.
func (s *Synth) GetAdlIns(insID uint16) adlData {
	if isDynamic(insID) {
		return s.dynamicIns(dyn(insID))
	}
	return s.staticIns(int(insID))
}

func (s *Synth) dynamicIns(idx int) adlData {
	if idx < 0 || idx >= len(s.dyno.ins) {
		return adlData{}
	}
	return s.dyno.ins[idx]
}

func (s *Synth) staticIns(idx int) adlData {
	if idx < 0 || idx >= len(adlIns) {
		return adlData{}
	}
	return adlIns[idx]
}
