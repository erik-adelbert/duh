// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) startSampleVoice(track TrackID, cmd ModCommand, sampleID int, note uint8, instrumentID int) bool {
	return w.startSampleVoiceWithNNA(track, cmd, sampleID, note, instrumentID, true)
}

type voiceStartState struct {
	carried           VoiceLifecycle
	previousVolume    float32
	hasPreviousVolume bool
	foregroundCut     bool
	nna               uint8
	hasNNA            bool
}

func (w *Tracker) startSampleVoiceWithNNA(track TrackID, cmd ModCommand, sampleID int, note uint8, instrumentID int, checkNNA bool) bool {
	state := w.prepareVoiceStart(track, cmd, sampleID, note, instrumentID, checkNNA)
	cursor := &w.TrackCursors[track]

	voice := w.allocVoice()
	if voice == -1 {
		return false
	}

	oplChannel := 0
	if w.Assets.Samples[sampleID].opl != nil {
		oplChannel = w.allocOPLVoice(voice)
		if oplChannel == 0 {
			return false
		}
	}

	w.initializeVoiceState(track, voice, cmd, sampleID, note, instrumentID, state)

	applyDefaultPan := !w.Assets.ITCompatibility.mptPitchPanSeparation ||
		w.Assets.ITCompatibility.mptPanningReset || cmd.Instrument != 0

	w.initializeVoicePanning(cursor, voice, sampleID, instrumentID, applyDefaultPan)

	cutoff, resonance := w.initializeInstrumentAndFilter(
		cursor, voice, sampleID, instrumentID, cmd.Note, state.carried, applyDefaultPan,
	)

	w.initializeVoicePlayback(
		track, voice, cmd, sampleID, instrumentID, oplChannel, cutoff, resonance,
	)

	return true
}

func (w *Tracker) prepareVoiceStart(track TrackID, cmd ModCommand, sampleID int, note uint8, instrumentID int, checkNNA bool) voiceStartState {
	cursor := &w.TrackCursors[track]

	var state voiceStartState

	if cmd.Instrument != 0 {
		cursor.mptMicroTuning = 0
	}

	if w.Assets.ITCompatibility.mptMixProfile != nil {
		if w.Assets.ITCompatibility.mptNNAReset || cmd.Instrument != 0 {
			cursor.mptHasNewNoteAction = false
		} else {
			state.nna = cursor.mptNewNoteAction
			state.hasNNA = cursor.mptHasNewNoteAction
		}
	}

	state.previousVolume, state.hasPreviousVolume = cursor.itLastVolume, cursor.itHasLastVolume

	if prev := cursor.ActiveVoice; prev != -1 && w.VoiceLives[prev].Active {
		state.previousVolume = w.VoiceAmps[prev].Volume
		state.hasPreviousVolume = true
	}

	if prev := cursor.ActiveVoice; prev != -1 && instrumentID != 0 && w.VoiceLives[prev].Active &&
		w.VoiceLives[prev].ParentTrack == track && w.VoiceLives[prev].ITInstrument == instrumentID &&
		(!w.VoiceLives[prev].KeyOff && !w.VoiceLives[prev].ITFade ||
			w.Assets.ITCompatibility.mptEnvelopeReset && w.VoiceLives[prev].FadeOutVol > 0 &&
				(!w.VoiceLives[prev].KeyOff || w.Assets.ITCompatibility.mptCarryAfterNoteOff && cmd.Note >= 1 && cmd.Note <= 120)) {
		state.carried = w.VoiceLives[prev]
	}

	if checkNNA {
		state.foregroundCut = w.applyITDuplicateNotes(
			track, sampleID, instrumentID, cmd.Note, note,
		)
	}

	if state.foregroundCut {
		state.carried = VoiceLifecycle{}
	}

	if checkNNA {
		w.applyNewNoteAction(track, cursor.ActiveVoice)
	} else if cursor.ActiveVoice != -1 {
		w.stopVoice(cursor.ActiveVoice)
		cursor.ActiveVoice = -1
	}

	if (w.Assets.ITCompatibility.itRestoreChannelPan ||
		w.Assets.ITCompatibility.mptLegacySwing ||
		w.Assets.ITCompatibility.mptPitchPanSeparation) &&
		cursor.itHasRestorePan {

		cursor.Panning = cursor.itRestorePan
		cursor.Surround = cursor.itRestoreSurround
		cursor.itHasRestorePan = false
	}

	return state
}

func (w *Tracker) initializeVoiceState(track TrackID, voice VoiceID, cmd ModCommand, sampleID int, note uint8, instrumentID int, state voiceStartState) {
	cursor := &w.TrackCursors[track]
	cursor.ActiveVoice = voice
	cursor.oplHasEndedPitch = false

	if !w.Assets.ITCompatibility.mptITRetrigger &&
		(w.Assets.ITCompatibility.mptMixProfile == nil || w.Assets.ITCompatibility.mptLegacyTremor) {
		w.TrackMemories[track].RetrigCount = 0
	}

	if w.Assets.s3m {
		w.TrackMemories[track].ITVibratoPos = 0
		w.TrackMemories[track].itTremoloPos = 0
	}

	if w.Assets.IT {
		if w.TrackMemories[track].itVibratoWaveform < 4 {
			w.TrackMemories[track].ITVibratoPos = 0
			if w.itLegacyModulationWaveform() && !w.Assets.ITCompatibility.OldEffects {
				w.TrackMemories[track].ITVibratoPos = 16
			}
		}

		if !w.Assets.ITCompatibility.itCompatibleTremor() && !w.Assets.ITCompatibility.mptITRetrigger {
			w.TrackMemories[track].itTremorCount = 0
		}

		if w.itLegacyModulationWaveform() {
			if w.TrackMemories[track].itTremoloWaveform < 4 {
				w.TrackMemories[track].itTremoloPos = 0
			}
		}
	}

	w.VoiceLives[voice] = VoiceLifecycle{
		Active:             true,
		ParentTrack:        track,
		FadeOutVol:         1,
		ITInstrument:       instrumentID,
		ITEnvelopeVolume:   1,
		ITFilterCutoff:     cursor.ITFilterCutoff,
		ITFilterResonance:  cursor.ITFilterResonance,
		itNewNoteAction:    state.nna,
		itHasNewNoteAction: state.hasNNA,
	}

	freq := noteToHz(note)
	if w.Assets.s3m {
		freq = s3mNoteFrequency(note, w.Assets.Samples[sampleID].C5Speed, w.Assets.s3mFrequencyPitch)
	} else if w.Assets.IT {
		freq = noteToHz(note - 1)
	}

	w.VoicePitches[voice] = PitchState{
		Note:           note,
		itRealNote:     cmd.Note,
		BaseFreqHz:     freq,
		mptMicroTuning: cursor.mptMicroTuning,
	}

	if w.Assets.IT && w.Assets.ITCompatibility.LegacyModPlug {
		freq, pitch := itLegacyNotePitch(
			note-1, &w.Assets.Samples[sampleID], w.Assets.ITCompatibility.LinearSlides,
		)
		w.VoicePitches[voice].BaseFreqHz, w.VoicePitches[voice].ITSlidePitch = freq, pitch

	}

	w.VoiceAmps[voice] = AmplitudeState{
		Volume:   float32(w.Assets.Samples[sampleID].Volume) / 64,
		Panning:  cursor.Panning,
		Surround: cursor.Surround,
	}

	if instrumentID != 0 && cmd.Instrument == 0 {
		switch {
		case state.foregroundCut:
			w.VoiceAmps[voice].Volume = 0
		case state.hasPreviousVolume:
			w.VoiceAmps[voice].Volume = state.previousVolume
		}
	}
}

func (w *Tracker) initializeVoicePanning(cursor *TrackCursor, voice VoiceID, sampleID, instrumentID int, applyDefaultPan bool) {
	sample := w.Assets.Samples[sampleID]
	if applyDefaultPan && w.Assets.IT && sample.ITHasPanning &&
		!w.Assets.ITCompatibility.itRestoreChannelPan {

		w.VoiceAmps[voice].Panning = float32(sample.ITPanning)/32 - 1
		cursor.Panning = w.VoiceAmps[voice].Panning

		if !w.Assets.ITCompatibility.LegacyModPlug {
			cursor.Surround = false
			w.VoiceAmps[voice].Surround = false
		}
	}

	if applyDefaultPan && w.Assets.ITCompatibility.itRestoreChannelPan {
		sample := &w.Assets.Samples[sampleID]

		switch {
		case sample.ITHasPanning:
			applyITDefaultPanning(
				cursor, &w.VoiceAmps[voice], float32(sample.ITPanning)/32-1,
			)
		case instrumentID != 0 && w.Assets.ITInstruments[instrumentID-1].HasPanning:
			applyITDefaultPanning(
				cursor, &w.VoiceAmps[voice],
				w.Assets.ITInstruments[instrumentID-1].normalizedPanning(),
			)
		}
	}
}

func (w *Tracker) initializeInstrumentAndFilter(cursor *TrackCursor, voice VoiceID, sampleID, instrumentID int, note uint8, carried VoiceLifecycle, applyDefaultPan bool) (uint8, uint8) {
	if instrumentID != 0 {
		initializeITInstrumentVoice(w,
			cursor, voice, sampleID, instrumentID, note, carried, applyDefaultPan,
		)
		w.VoiceLives[voice].ITFilterCutoff = cursor.ITFilterCutoff
		w.VoiceLives[voice].ITFilterResonance = cursor.ITFilterResonance
		if swing := w.Assets.ITInstruments[instrumentID-1].mptCutoffSwing; swing != 0 {
			w.VoiceLives[voice].ITFilterCutoff = mptRandomCutoff(
				cursor.ITFilterCutoff, swing, w.itRandom.Intn(256)-128,
			)
		}

		if swing := w.Assets.ITInstruments[instrumentID-1].mptResonanceSwing; swing != 0 {
			life := &w.VoiceLives[voice]
			delta := mptFilterSwingDelta(cursor.ITFilterResonance, swing, w.itRandom.Intn(256)-128)
			life.ITFilterResonance = mptApplyFilterSwing(cursor.ITFilterResonance, delta)
			if w.Assets.ITCompatibility.mptOldSwingBehaviour {
				life.mptResonanceSwing = delta
			} else {
				life.mptRestoreResonance = true
			}
		}
	}

	cutoff := cursor.ITFilterCutoff
	resonance := cursor.ITFilterResonance
	if instrumentID != 0 {
		cutoff = w.VoiceLives[voice].ITFilterCutoff
		resonance = w.VoiceLives[voice].ITFilterResonance
	}

	w.VoiceFilters[voice] = newITFilter(
		cutoff, resonance,
		w.Clock.SampleRate,
		w.Assets.IT && (instrumentID != 0 || w.Assets.ITCompatibility.mptMixProfile != nil || w.Assets.ITCompatibility.mptMacroConfig != nil),
		w.Assets.ITCompatibility.LegacyModPlug,
		w.Assets.ITCompatibility.ExtendedFilter,
	)
	w.VoiceLives[voice].mptHighPass = cursor.mptHighPass

	setITFilterMode(&w.VoiceFilters[voice], cursor.mptHighPass)
	w.VoiceRightFilters[voice] = w.VoiceFilters[voice]

	return cutoff, resonance
}

func (w *Tracker) initializeVoicePlayback(track TrackID, voice VoiceID, cmd ModCommand, sampleID, instrumentID, oplChannel int, cutoff, resonance uint8) {
	offset := 0

	switch {
	case cmd.EffectCmd == 0x12:
		if cmd.EffectArg != 0 {
			w.TrackMemories[track].LastOffset = cmd.EffectArg
		}

		offset = int(w.TrackMemories[track].LastOffset) * 256
		if w.Assets.s3m {
			offset += int(w.TrackMemories[track].ITHighOffset) << 16
		}
		if w.Assets.IT {
			offset = itSampleOffset(w, int(track), &w.Assets.Samples[sampleID])
		}
	case w.Assets.ITCompatibility.mptMixProfile != nil && isMPTCueOffset(cmd) && oplChannel == 0:
		sample := &w.Assets.Samples[sampleID]

		offset = clampITSampleOffset(
			int(sample.mptCues[cmd.Volume-224]), sample,
			!w.Assets.ITCompatibility.mptCueOffsetITBehaviour,
			w.Assets.ITCompatibility.OldEffects,
		)
	}

	w.VoiceSamples[voice] = SamplePlayback{
		SampleID:        sampleID,
		Position:        float64(offset),
		ScratchBuf:      w.VoiceSamples[voice].ScratchBuf,
		RightScratchBuf: w.VoiceSamples[voice].RightScratchBuf,

		oplChannel: oplChannel,
	}

	if oplChannel != 0 {
		w.opl.setPatch(oplChannel-1, *w.Assets.Samples[sampleID].opl)

		if (w.Assets.ITCompatibility.mptMixProfile != nil ||
			instrumentID != 0 && w.Assets.ITInstruments[instrumentID-1].mptCutoffSwing != 0) &&
			(cutoff < 127 || resonance != 0) {
			w.opl.setModulatorCutoff(oplChannel-1, cutoff/2)
		}
	}
}
