// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

func nextITOrder(orders []int, order int) int {
	for order < len(orders) && orders[order] == itOrderSkip {
		order++
	}
	return order
}

// System 1: Advances ticks, rows, and orders; calculates exact samples per tick.
func TickSchedulerSystem(w *Tracker) bool {
	c := &w.Clock
	if c.SamplesRemaining > 0 {
		return false // Still inside current tick; skip control systems
	}

	c.Tick++
	if c.Tick >= w.mptRowTicks() {
		c.Tick = 0
		w.mptRowDelay = 0
		w.mptFrameDelay = 0
		if w.Assets.IT && w.itFlowPending {
			if w.itFlowBreak && !w.loopProfile().preserveOnBreak && c.Order != w.itNextOrder {
				for t := 0; t < w.Assets.NumTracks; t++ {
					w.TrackMemories[t].itLoopCount = 0
				}
			}
			c.Order, c.Row = w.itNextOrder, w.itNextRow
			w.itFlowPending = false
			if c.Order >= len(w.Assets.OrderList) {
				c.SongFinished = true
				return false
			}
		} else {
			c.Row++
		}

		c.Order = nextITOrder(w.Assets.OrderList, c.Order)
		if c.Order >= len(w.Assets.OrderList) {
			c.SongFinished = true
			return false
		}

		patIdx := w.Assets.OrderList[c.Order]
		if c.Row >= len(w.Assets.Patterns[patIdx].Rows) {
			c.Row = 0
			c.Order++
			c.Order = nextITOrder(w.Assets.OrderList, c.Order)
			if c.Order >= len(w.Assets.OrderList) {
				c.SongFinished = true
				return false
			}
		}
	}

	if c.Tick == 0 {
		c.Order = nextITOrder(w.Assets.OrderList, c.Order)
		if c.Order >= len(w.Assets.OrderList) {
			c.SongFinished = true
			return false
		}
	}

	if c.Tick == 0 && w.mptVisitedRows != nil {
		state := mptVisitedRow{order: c.Order, row: c.Row}
		for track := 0; track < w.Assets.NumTracks; track++ {
			state.loopCounts[track] = w.TrackMemories[track].itLoopCount
		}

		if _, visited := w.mptVisitedRows[state]; visited {
			c.SongFinished = true
			return false
		}
		w.mptVisitedRows[state] = struct{}{}
	}

	if c.Tick == 0 && w.Assets.mptRowsPerBeat != 0 {
		pattern := &w.Assets.Patterns[w.Assets.OrderList[c.Order]]
		c.mptRowsPerBeat = uint32(w.Assets.mptRowsPerBeat)
		if override := pattern.mptRowsPerBeat; override != 0 {
			c.mptRowsPerBeat = override
		}

		c.mptSwingFactor = 1 << 24
		swing := w.Assets.mptTempoSwing
		if len(pattern.mptTempoSwing) != 0 {
			swing = pattern.mptTempoSwing
		}

		if len(swing) != 0 {
			c.mptSwingFactor = swing[c.Row%len(swing)]
		}
	}

	// Tick zero is finalized after row speed and tempo commands have been applied.
	commit := c.Tick != 0
	if commit && !w.Assets.ITCompatibility.mptTempoAsSpeed {
		for t := 0; t < w.Assets.NumTracks; t++ {
			if w.PendingCmds[t].EffectCmd == 0x20 {
				commit = false
				break
			}
		}
	}

	c.SamplesPerTick = samplesPerTick(c, commit)
	c.SamplesRemaining = c.SamplesPerTick
	return true
}

func samplesPerTick(c *SequencerClock, commit bool) int {
	if c.mptRowsPerBeat == 0 {
		return int(c.SampleRate * 2.5 / c.Tempo)
	}

	exact := c.SampleRate * 60 / (c.Tempo * float64(c.Speed) * float64(c.mptRowsPerBeat))
	exact *= float64(c.mptSwingFactor) / (1 << 24)
	frames := int(exact)

	remainder := c.mptTickRemainder + exact - float64(frames)
	if remainder >= 1 {
		frames++
		remainder--
	}

	if commit {
		c.mptTickRemainder = remainder
	}

	return max(frames, 1)
}

// System 2 (Tick 0 only): Fetches pattern row cells into Track components.
func PatternFetchSystem(w *Tracker) {
	if w.Clock.Tick != 0 {
		return
	}

	patIdx := w.Assets.OrderList[w.Clock.Order]
	rowCells := w.Assets.Patterns[patIdx].Rows[w.Clock.Row]

	for t := 0; t < w.Assets.NumTracks; t++ {
		cmd := rowCells[t]
		if w.Assets.s3m {
			mem := &w.TrackMemories[t]
			if cmd.EffectCmd == 0x15 && cmd.EffectArg == 0 {
				cmd.EffectArg = mem.ITLastSpecial
			}
			if w.Assets.s3mSharedEffectMemory && (cmd.EffectCmd != 0 || cmd.s3mDummy) && cmd.EffectArg != 0 {
				mem.LastVolSlide = cmd.EffectArg
				mem.LastPortaUp, mem.LastPortaDown = cmd.EffectArg, cmd.EffectArg
				mem.itTremorParam, mem.LastArpeggio = cmd.EffectArg, cmd.EffectArg
				mem.LastRetrig, mem.itTremolo, mem.ITLastSpecial = cmd.EffectArg, cmd.EffectArg, cmd.EffectArg
			} else if cmd.EffectCmd == 0x15 && cmd.EffectArg != 0 {
				mem.ITLastSpecial = cmd.EffectArg
			}
		}
		if w.Assets.ITCompatibility.mptEmptyNoteMapIgnoreCell &&
			cmd.Note >= 1 && cmd.Note <= 120 && cmd.Instrument != 0 &&
			int(cmd.Instrument) <= len(w.Assets.ITInstruments) &&
			w.Assets.ITInstruments[cmd.Instrument-1].Samples[cmd.Note-1] == 0 {
			w.TrackMemories[t].LastNote = cmd.Note
			w.TrackMemories[t].itSpecialNote = false
			w.PendingCmds[t] = ModCommand{}
			continue
		}
		w.PendingCmds[t] = cmd
	}
}

// System 3: Triggers notes on their scheduled tick.
func VoiceAllocationSystem(w *Tracker) {
	for t := 0; t < w.Assets.NumTracks; t++ {
		w.allocateTrackVoice(t, w.PendingCmds[t])
	}
}

func (w *Tracker) allocateTrackVoice(t int, cmd ModCommand) {
	cursor := &w.TrackCursors[t]
	if w.Assets.IT && w.Assets.ITMuted[t] {
		return
	}

	if cmd.EffectCmd == 0x25 &&
		w.Clock.Tick == int(cmd.EffectArg>>4)+int(cmd.EffectArg&15) &&
		cursor.ActiveVoice != -1 &&
		w.VoiceLives[cursor.ActiveVoice].Active {

		if w.Assets.ITCompatibility.mptCutStopsSample {
			w.stopVoice(cursor.ActiveVoice)
		} else {
			w.VoiceAmps[cursor.ActiveVoice].Volume = 0
		}
	}

	triggerTick := 0

	if w.Assets.IT {
		triggerTick = itNoteTriggerTick(cmd, &w.TrackMemories[t], w.Assets.ITCompatibility.LegacyModPlug)
		if triggerTick >= w.Clock.Speed && w.Clock.Tick == 0 && cmd.Instrument != 0 && !w.Assets.ITCompatibility.LegacyModPlug {
			w.TrackMemories[t].LastInstrument = cmd.Instrument
		}
	} else if cmd.EffectCmd == 0x15 && cmd.EffectArg&0xf0 == 0xd0 {
		triggerTick = int(cmd.EffectArg & 0x0f)
	}

	if w.Clock.Tick != triggerTick {
		return
	}

	if w.Assets.IT && cmd.EffectCmd == 0x12 && cmd.EffectArg != 0 {
		w.TrackMemories[t].LastOffset = cmd.EffectArg
	}

	if cmd.Note >= 1 && cmd.Note <= 120 {
		w.TrackMemories[t].LastNote = cmd.Note
		w.TrackMemories[t].itSpecialNote = false
		if w.Assets.IT {
			w.TrackMemories[t].itPanbrelloOffset = 0
		}
	} else if w.Assets.IT && cmd.Note >= 253 {
		w.TrackMemories[t].itSpecialNote = true
	}

	if w.applySpecialNote(t, cmd.Note) {
		if w.Assets.IT && w.Assets.ITCompatibility.mptMixProfile == nil && itFadeDirectVolume(cmd) {
			w.applyDirectVolumeColumn(TrackID(t), cmd)
		}
		return
	}

	w.applyInstrumentCommand(t, cmd)
	tonePortamento := cmd.EffectCmd == 0x03 || w.Assets.s3m && cmd.EffectCmd == 0x1f || w.Assets.IT && itHasTonePortamento(cmd)
	retriggerPortaSwap := w.preparePortamentoNote(cursor, cmd, tonePortamento)
	if cmd.Note >= 1 && cmd.Note <= 120 && (!tonePortamento ||
		w.Assets.IT && (retriggerPortaSwap || cursor.ActiveVoice == -1 || !w.VoiceLives[cursor.ActiveVoice].Active) ||
		w.Assets.s3m && (cursor.ActiveVoice == -1 || !w.VoiceLives[cursor.ActiveVoice].Active)) {

		if !w.triggerTrackNote(t, cmd, tonePortamento) {
			return
		}
	}

	w.applyDirectVolumeColumn(TrackID(t), cmd)
}

func (w *Tracker) applySpecialNote(t int, note uint8) bool {
	cursor := &w.TrackCursors[t]

	switch {
	case note == 253 && w.Assets.IT:
		if cursor.ActiveVoice != -1 {
			life := &w.VoiceLives[cursor.ActiveVoice]
			if life.ITInstrument != 0 {
				life.ITFade = true
			}
			if channel := w.VoiceSamples[cursor.ActiveVoice].oplChannel; channel != 0 &&
				!w.Assets.ITCompatibility.oplFlexibleNoteOff {
				w.opl.noteOff(channel - 1)
				life.itOPLLegacyFade = life.ITInstrument != 0
			}
		}
		return true
	case note == 254 && (cursor.ActiveVoice != -1 || cursor.oplHasEndedPitch):
		cursor.oplHasEndedPitch = false
		if cursor.ActiveVoice != -1 {
			w.stopVoice(cursor.ActiveVoice)
			cursor.ActiveVoice = -1
		}
		if w.Assets.IT {
			w.TrackMemories[t].ITNoteCut = w.Assets.ITCompatibility.LegacyModPlug
		}
		return true
	case note == 255 && cursor.ActiveVoice != -1:
		if w.VoiceLives[cursor.ActiveVoice].ITInstrument != 0 {
			releaseITVoice(w, cursor.ActiveVoice)
		} else if w.VoiceSamples[cursor.ActiveVoice].oplChannel != 0 {
			w.VoiceLives[cursor.ActiveVoice].KeyOff = true
		} else if w.Assets.IT {
			w.stopVoice(cursor.ActiveVoice)
			cursor.ActiveVoice = -1
		} else {
			w.VoiceLives[cursor.ActiveVoice].KeyOff = true
		}
		return true
	}
	return false
}

func (w *Tracker) applyInstrumentCommand(t int, cmd ModCommand) {
	cursor := &w.TrackCursors[t]

	if cmd.Instrument != 0 {
		previousInstrument := w.TrackMemories[t].LastInstrument
		w.TrackMemories[t].LastInstrument = cmd.Instrument

		if w.Assets.ITCompatibility.oplInstrumentSwitch && cmd.Note == 0 {
			w.switchITInstrumentOnly(TrackID(t), cmd, previousInstrument)
		} else if w.Assets.IT && len(w.Assets.ITInstruments) != 0 &&
			w.Assets.ITCompatibility.mptMixProfile == nil && cmd.Note == 0 {
			w.applyITInstrumentOnly(TrackID(t), cmd, previousInstrument)
		}

		sample := int(cmd.Instrument) - 1
		if len(w.Assets.ITInstruments) == 0 && cmd.Note == 0 && cursor.ActiveVoice != -1 && w.VoiceLives[cursor.ActiveVoice].Active && sample < len(w.Assets.Samples) {
			w.VoiceAmps[cursor.ActiveVoice].Volume = float32(w.Assets.Samples[sample].Volume) / 64
		}
		w.restoreITCutSample(TrackID(t), cmd)
	}

	if w.Assets.IT && !w.Assets.ITCompatibility.LegacyModPlug && cmd.EffectCmd == 0x12 &&
		(w.Assets.ITCompatibility.mptMixProfile == nil || w.Assets.ITCompatibility.mptOffsetWithInstrument &&
			w.TrackMemories[t].LastNote != 0 && (!w.TrackMemories[t].itSpecialNote ||
			!w.Assets.ITCompatibility.mptClearOldNoteAfterCut)) && cmd.Note == 0 && cmd.Instrument != 0 &&
		cursor.ActiveVoice != -1 && w.VoiceLives[cursor.ActiveVoice].Active {
		pb := &w.VoiceSamples[cursor.ActiveVoice]
		instrument := w.VoiceLives[cursor.ActiveVoice].ITInstrument
		note := w.TrackMemories[t].LastNote
		if w.Assets.ITCompatibility.mptMixProfile == nil || instrument == 0 || note != 0 && instrument <= len(w.Assets.ITInstruments) &&
			w.Assets.ITInstruments[instrument-1].Samples[note-1] != 0 &&
			int(w.Assets.ITInstruments[instrument-1].Samples[note-1]) <= len(w.Assets.Samples) {
			pb.Position = float64(itSampleOffset(w, t, &w.Assets.Samples[pb.SampleID]))
			pb.mptLooped = false
		}
	}
}

func (w *Tracker) preparePortamentoNote(cursor *TrackCursor, cmd ModCommand, tonePortamento bool) bool {
	if tonePortamento && cmd.Note >= 1 && cmd.Note <= 120 && cursor.ActiveVoice != -1 {
		life := &w.VoiceLives[cursor.ActiveVoice]

		if life.Active && (w.Assets.ITCompatibility.mptLegacySwing || w.Assets.ITCompatibility.mptPitchPanSeparation) && cursor.itHasRestorePan {
			cursor.Panning, cursor.Surround = cursor.itRestorePan, cursor.itRestoreSurround
			cursor.itHasRestorePan = false
			w.VoiceAmps[cursor.ActiveVoice].Panning = cursor.Panning
			w.VoiceAmps[cursor.ActiveVoice].Surround = cursor.Surround
		}

		if life.Active && life.mptRestoreResonance {
			life.ITFilterResonance = cursor.ITFilterResonance
			life.mptRestoreResonance = false
		}
	}

	retrigger := w.applyITPortamentoSampleChange(cursor, cmd)

	if tonePortamento && cmd.Note >= 1 && cmd.Note <= 120 && !retrigger &&
		w.Assets.ITCompatibility.mptPitchPanSeparation && cursor.ActiveVoice != -1 &&
		w.VoiceLives[cursor.ActiveVoice].Active && w.VoiceLives[cursor.ActiveVoice].ITInstrument != 0 {
		voice := cursor.ActiveVoice

		if w.Assets.ITCompatibility.mptPanningReset {
			sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]
			instrument := &w.Assets.ITInstruments[w.VoiceLives[voice].ITInstrument-1]
			if sample.ITHasPanning {
				w.applyMPTNoteDefaultPan(cursor, voice, float32(sample.ITPanning)/32-1)
			} else if instrument.HasPanning {
				w.applyMPTNoteDefaultPan(cursor, voice, instrument.normalizedPanning())
			}
		}
		w.applyMPTPitchPan(cursor, cursor.ActiveVoice, cmd.Note)
	}

	return retrigger
}

func (w *Tracker) triggerTrackNote(t int, cmd ModCommand, tonePortamento bool) bool {
	cursor := &w.TrackCursors[t]

	inst := cmd.Instrument
	if inst == 0 {
		inst = w.TrackMemories[t].LastInstrument
	}

	sampID, note, instrumentID, ok := resolveSampleNote(&w.Assets, inst, cmd.Note)
	if !ok {
		if len(w.Assets.ITInstruments) != 0 && int(inst) > len(w.Assets.ITInstruments) {
			w.applyDirectVolumeColumn(TrackID(t), cmd)
		}
		return false
	}

	restoreOPLPitch := tonePortamento && cmd.Instrument == 0 && cursor.ActiveVoice == -1 && cursor.oplHasEndedPitch &&
		cursor.oplEndedSample >= 0 && cursor.oplEndedSample < len(w.Assets.Samples)
	endedPitch := cursor.oplEndedPitch
	endedSample := cursor.oplEndedSample
	if !w.startVoiceAfterITCut(TrackID(t), cmd, sampID, note, instrumentID) {
		return false
	}

	if restoreOPLPitch {
		pitch := &w.VoicePitches[cursor.ActiveVoice]
		pitch.BaseFreqHz = endedPitch.BaseFreqHz * float64(w.Assets.Samples[endedSample].C5Speed) /
			float64(w.Assets.Samples[sampID].C5Speed)
		pitch.ITSlidePitch = endedPitch.ITSlidePitch
	}

	return true
}

func (w *Tracker) applyDirectVolumeColumn(track TrackID, cmd ModCommand) {
	voice := w.TrackCursors[track].ActiveVoice

	if !cmd.HasVolume || voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	switch {
	case cmd.Volume <= 64:
		w.VoiceAmps[voice].Volume = float32(cmd.Volume) / 64
	case cmd.Volume >= 128 && cmd.Volume <= 192:
		setTrackPanning(w, track, voice, float32(cmd.Volume-128)/32-1)
	}
}

func resolveSampleNote(assets *SongAssets, instrument, note uint8) (sampleID int, mappedNote uint8, instrumentID int, ok bool) {
	sampleID, mappedNote = int(instrument)-1, note

	if len(assets.ITInstruments) > 0 {
		if instrument == 0 || int(instrument) > len(assets.ITInstruments) {
			return 0, 0, 0, false
		}

		instrumentID = int(instrument)
		mapping := &assets.ITInstruments[instrumentID-1]
		sampleID = int(mapping.Samples[note-1]) - 1
		mappedNote = mapping.Notes[note-1] + 1
	}

	return sampleID, mappedNote, instrumentID,
		sampleID >= 0 && sampleID < len(assets.Samples) &&
			(len(assets.Samples[sampleID].Data) >= 2 || assets.Samples[sampleID].opl != nil)
}

func (w *Tracker) applyNewNoteAction(track TrackID, voice VoiceID) {
	if voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	life := &w.VoiceLives[voice]
	if life.ITInstrument == 0 {
		w.stopVoice(voice)
		return
	}

	action := w.Assets.ITInstruments[life.ITInstrument-1].NewNoteAction
	if life.itHasNewNoteAction {
		action = life.itNewNoteAction
	}

	switch action {
	case 0:
		w.stopVoice(voice)
	case 2:
		releaseITVoice(w, voice)
	case 3:
		life.ITFade = true
	}

	if life.Active {
		life.ITPastChannelVolume = w.ITChannelVolume[track]
		life.ITPastNote = true
		w.VoicePitches[voice].VibratoOffset = 0
		w.VoiceAmps[voice].itPanbrelloOffset = 0
		w.VoiceAmps[voice].itTremorMuted = false
		w.VoiceAmps[voice].itTremoloOffset = 0
	}
}

func itNoteTriggerTick(cmd ModCommand, mem *EffectMemory, legacy bool) int {
	if cmd.EffectCmd == 0x25 {
		return int(cmd.EffectArg >> 4)
	}

	if cmd.EffectCmd != 0x15 {
		return 0
	}

	arg := cmd.EffectArg
	if arg == 0 {
		arg = mem.ITLastSpecial
	}

	if arg&0xf0 == 0xd0 {
		delay := int(arg & 0x0f)
		if delay == 0 && !legacy {
			return 1
		}
		return delay
	}
	return 0
}

func itSampleOffset(w *Tracker, track int, sample *SampleAsset) int {
	mem := &w.TrackMemories[track]
	offset := int(mem.LastOffset)*256 + int(mem.ITHighOffset)<<16
	return clampITSampleOffset(offset, sample, w.Assets.ITCompatibility.LegacyModPlug, w.Assets.ITCompatibility.OldEffects)
}

func clampITSampleOffset(offset int, sample *SampleAsset, legacy, oldEffects bool) int {
	end := len(sample.Data)
	if sample.mptSustainLoop {
		end = min(end, int(sample.mptSustainEnd))
	} else if sample.LoopEnable {
		end = min(end, int(sample.LoopEnd))
	}

	if offset < end {
		return offset
	}

	switch {
	case legacy && oldEffects && end > 4:
		return end - 2
	case legacy:
		return int(sample.LoopStart)
	case oldEffects:
		return end
	default:
		return 0
	}
}

func initializeITInstrumentVoice(w *Tracker, cursor *TrackCursor, voice VoiceID, sampleID, instrumentID int, note uint8, carried VoiceLifecycle, applyDefaultPan bool) {
	instrument := &w.Assets.ITInstruments[instrumentID-1]
	sample := &w.Assets.Samples[sampleID]
	life, amp := &w.VoiceLives[voice], &w.VoiceAmps[voice]

	if instrument.HasFilterCutoff {
		cursor.ITFilterCutoff = instrument.FilterCutoff
	}

	if instrument.HasFilterResonance {
		cursor.ITFilterResonance = instrument.FilterResonance
	}

	if instrument.mptHasFilterMode {
		cursor.mptHighPass = instrument.mptFilterMode == 1
	}

	if instrument.VolumeEnvelope.Carry {
		life.ITEnvelopeTick = carried.ITEnvelopeTick
		life.mptEnvelopePosition = carried.mptEnvelopePosition
	}

	if instrument.PanEnvelope.Carry {
		life.ITPanEnvelopeTick = carried.ITPanEnvelopeTick
	}

	if instrument.PitchEnvelope.Carry {
		life.ITPitchEnvelopeTick = carried.ITPitchEnvelopeTick
	}

	if applyDefaultPan && instrument.HasPanning && !w.Assets.ITCompatibility.itRestoreChannelPan &&
		(!w.Assets.ITCompatibility.mptPitchPanSeparation || !sample.ITHasPanning) {
		amp.Panning = instrument.normalizedPanning()
		cursor.Panning = amp.Panning
		if !w.Assets.ITCompatibility.LegacyModPlug {
			cursor.Surround = false
			amp.Surround = false
		}
	}

	legacy := w.Assets.ITCompatibility.LegacyModPlug
	if w.Assets.ITCompatibility.mptPitchPanSeparation {
		w.applyMPTPitchPan(cursor, voice, note)
	} else if !legacy {
		amp.Panning = itPitchPan(amp.Panning, note, instrument, false)
	}

	amp.ITInstrumentVolume = itInstrumentSampleVolume(sample, instrument)
	volSwing, panSwing := int(instrument.VolumeSwing), int(instrument.PanSwing)
	if legacy {
		volSwing = min(volSwing*100/64, 100)
		panSwing = (panSwing + 3) / 4
	}

	if volSwing != 0 {
		base := int(sample.ITGlobalVolume) * int(instrument.GlobalVolume/2) / 64
		if !w.Assets.ITCompatibility.mptLegacySwing {
			amp.ITInstrumentVolume = float32(base) / 64
		}

		if legacy {
			base = (int(sample.Volume)*4 + 1) / 2
		} else if w.Assets.ITCompatibility.mptLegacySwing {
			base = (int(amp.Volume*256+0.5) + 1) / 2
		}
		amp.ITVolumeSwing = int16(((w.itRandom.Intn(256)-128)*volSwing/64 + 1) * base / 199)
	}

	if panSwing != 0 {
		amp.ITPanSwing = int16((w.itRandom.Intn(256) - 128) * panSwing * 4 / 128)
		if w.Assets.ITCompatibility.mptLegacySwing && !cursor.itHasRestorePan {
			cursor.itRestorePan, cursor.itRestoreSurround = cursor.Panning, cursor.Surround
			cursor.itHasRestorePan = true
		}
	}

	amp.mptApplyLegacySwing = w.Assets.ITCompatibility.mptLegacySwing && (volSwing != 0 || panSwing != 0)
}

func releaseITVoice(w *Tracker, v VoiceID) {
	life := &w.VoiceLives[v]
	if !life.KeyOff && !life.mptSustainSuppressed && w.VoiceSamples[v].SampleID >= 0 && w.VoiceSamples[v].SampleID < len(w.Assets.Samples) &&
		w.Assets.Samples[w.VoiceSamples[v].SampleID].mptSustainLoop {
		pb := &w.VoiceSamples[v]
		sample := &w.Assets.Samples[pb.SampleID]
		pb.mptLooped = false
		if sample.LoopEnable && sample.LoopEnd > sample.LoopStart {
			if int(pb.Position) > int(sample.LoopEnd) {
				pb.Position = float64(int(sample.LoopStart) + (int(pb.Position)-int(sample.LoopStart))%int(sample.LoopEnd-sample.LoopStart))
			}
			if !sample.LoopPingPong {
				pb.Reverse = false
			}
		} else {
			pb.Reverse = false
		}
	}
	life.KeyOff = true
	instrument := &w.Assets.ITInstruments[life.ITInstrument-1]
	if instrument.mptHasVolumeRelease && !life.mptEnvelopeReleased {
		env := &instrument.VolumeEnvelope
		position := life.ITEnvelopeTick
		if w.Assets.ITCompatibility.mptEnvelopePositionHandling {
			position = life.mptEnvelopePosition
		}
		life.mptReleaseValue = mptVolumeEnvelopeValue(env, position)
		life.mptEnvelopeReleased = true
		life.ITEnvelopeTick = int(env.Points[instrument.mptVolumeReleaseNode].Tick)
	}
	if !itEnvelopeEnabled(instrument.VolumeEnvelope.Enabled, life.itVolumeEnvelopeControl) || instrument.VolumeEnvelope.Loop && instrument.fadeoutStep() != 0 {
		life.ITFade = true
	}
}

// GlobalFlowSystem handles speed, tempo, order jumps, pattern breaks and IT pattern loops.
func GlobalFlowSystem(w *Tracker) {
	c := &w.Clock

	if !w.Assets.ITCompatibility.mptTempoAsSpeed {
		tempoCommand := false
		for t := 0; t < w.Assets.NumTracks; t++ {
			cmd := w.PendingCmds[t]
			if cmd.EffectCmd != 0x20 {
				continue
			}
			tempoCommand = true
			arg := cmd.EffectArg
			mem := &w.TrackMemories[t].itTempoSlide
			if arg != 0 {
				*mem = arg
			} else {
				arg = *mem
			}
			effectTick := w.itRepeatedRowTick()
			if arg >= 0x20 && effectTick == 0 {
				c.Tempo = float64(arg)
			} else if arg < 0x20 && effectTick != 0 {
				delta := float64(arg & 15)
				if arg < 0x10 {
					delta = -delta
				}
				maxTempo := 255.0
				if w.Assets.ITCompatibility.mptTempoUnclamped {
					maxTempo = 1000
				}
				c.Tempo = max(32, min(maxTempo, c.Tempo+delta))
			}
		}
		if tempoCommand && c.Tick != 0 {
			c.SamplesPerTick = samplesPerTick(c, true)
			c.SamplesRemaining = c.SamplesPerTick
		}
	}

	// 1. On Tick 0: Apply Speed (Axx) and Tempo (Txx) immediately
	if c.Tick == 0 {
		w.itLoopRow, w.itLoopChannel = -1, -1
		rowDelaySet := false
		for t := 0; t < w.Assets.NumTracks; t++ {
			cmd := w.PendingCmds[t]
			switch cmd.EffectCmd {
			case 0x0F: // S3M 'Axx' - Set Speed (ticks per row)
				if cmd.EffectArg > 0 {
					c.Speed = int(cmd.EffectArg)
				}
			case 0x20:
				if w.Assets.ITCompatibility.mptTempoAsSpeed && cmd.EffectArg > 0 {
					c.Speed = int(cmd.EffectArg)
				}
			case 0x14: // S3M 'Txx' - Set Tempo (BPM)
				if cmd.EffectArg >= 0x20 {
					c.Tempo = float64(cmd.EffectArg)
				}
			case 0x15:
				if w.Assets.IT {
					arg := cmd.EffectArg
					if arg == 0 {
						arg = w.TrackMemories[t].ITLastSpecial
					}
					if arg&0xf0 == 0xb0 {
						w.itPatternLoop(t, arg&15)
					}
					switch arg & 0xf0 {
					case 0x60:
						if w.Assets.ITCompatibility.mptMixProfile != nil {
							w.mptFrameDelay += int(arg & 15)
						}
					case 0xe0:
						if !rowDelaySet {
							w.mptRowDelay = int(arg & 15)
							rowDelaySet = true
						}
					}
				} else if w.Assets.s3m {
					switch cmd.EffectArg & 0xf0 {
					case 0x60:
						w.mptFrameDelay += int(cmd.EffectArg & 15)
					case 0xb0:
						w.itPatternLoop(t, cmd.EffectArg&15)
					case 0xe0:
						if !rowDelaySet {
							w.mptRowDelay = int(cmd.EffectArg & 15)
							rowDelaySet = true
						}
					}
				}
			}
		}

		// Recalculate tick duration after potential Speed/Tempo change
		c.SamplesPerTick = samplesPerTick(c, true)
		c.SamplesRemaining = c.SamplesPerTick
	}

	// On the last tick, schedule pattern flow changes.
	if c.Tick == w.mptRowTicks()-1 {
		if w.Assets.IT {
			w.scheduleITFlow()
			return
		}
		jumpOrder := -1
		breakRow := -1

		for t := 0; t < w.Assets.NumTracks; t++ {
			cmd := w.PendingCmds[t]
			switch cmd.EffectCmd {
			case 0x0B: // S3M 'Bxx' - Order Jump
				jumpOrder = int(cmd.EffectArg)
			case 0x0D: // S3M 'Cxx' - Pattern Break (BCD row in S3M)
				breakRow = int(cmd.EffectArg>>4)*10 + int(cmd.EffectArg&0x0F)
			}
		}

		if jumpOrder != -1 || breakRow != -1 {
			if jumpOrder != -1 {
				c.Order = jumpOrder
			} else {
				c.Order++
			}
			c.Order = nextITOrder(w.Assets.OrderList, c.Order)
			if c.Order >= len(w.Assets.OrderList) {
				c.SongFinished = true
				return
			}
			if breakRow != -1 && breakRow < len(w.Assets.Patterns[w.Assets.OrderList[c.Order]].Rows) {
				c.Row = breakRow - 1 // TickSchedulerSystem will increment this to breakRow
			} else {
				c.Row = -1 // TickSchedulerSystem will increment this to 0
			}
		} else if w.Assets.s3m && w.itLoopChannel >= 0 {
			c.Row = w.itLoopRow - 1
		}
	}
}

func (w *Tracker) stopVoice(voice VoiceID) {
	life := &w.VoiceLives[voice]
	if channel := w.VoiceSamples[voice].oplChannel; channel != 0 {
		w.opl.noteOff(channel - 1)
		w.opl.setVolume(channel-1, 0)
		w.oplVoices[channel-1] = -1
		w.VoiceSamples[voice].oplChannel = 0
	}
	life.Active = false
	track := life.ParentTrack
	if track >= 0 && int(track) < len(w.TrackCursors) && w.TrackCursors[track].ActiveVoice == voice {
		if w.Assets.IT && life.ITInstrument != 0 {
			w.TrackCursors[track].itLastVolume = w.VoiceAmps[voice].Volume
			w.TrackCursors[track].itHasLastVolume = true
		}
		w.TrackCursors[track].ActiveVoice = -1
	}
}

func noteToHz(note uint8) float64 {
	// MIDI-like note 60 (C-5) = 8363 Hz base tracker rate
	return 8363.0 * math.Pow(2.0, (float64(note)-60.0)/12.0)
}

func clamp32(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
