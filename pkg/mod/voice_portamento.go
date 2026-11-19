package mod

func (w *Tracker) applyITPortamentoSampleChange(cursor *TrackCursor, cmd ModCommand) bool {
	if !w.Assets.IT || !itHasTonePortamento(cmd) || cmd.Instrument == 0 || cmd.Note < 1 || cmd.Note > 120 ||
		cursor.ActiveVoice == -1 || !w.VoiceLives[cursor.ActiveVoice].Active {
		return false
	}
	sampleID := int(cmd.Instrument) - 1
	if len(w.Assets.ITInstruments) > 0 {
		sampleID = -1
		if int(cmd.Instrument) <= len(w.Assets.ITInstruments) {
			sampleID = int(w.Assets.ITInstruments[cmd.Instrument-1].Samples[cmd.Note-1]) - 1
		}
	}
	if sampleID < 0 || sampleID >= len(w.Assets.Samples) ||
		(len(w.Assets.Samples[sampleID].Data) < 2 && w.Assets.Samples[sampleID].opl == nil) {
		return false
	}
	voice := cursor.ActiveVoice
	pb := &w.VoiceSamples[voice]
	life := &w.VoiceLives[voice]
	if sampleID != pb.SampleID && w.Assets.ITCompatibility.CompatibleGxx &&
		w.Assets.ITCompatibility.mptPortamentoInstrument {
		sampleID = pb.SampleID
	}
	sampleChanged := sampleID != pb.SampleID
	if sampleChanged && w.Assets.ITCompatibility.mptMixProfile != nil &&
		!w.Assets.ITCompatibility.mptPortamentoSwapResetsPos {
		w.stopVoice(voice)
		return true
	}
	instrumentChanged := len(w.Assets.ITInstruments) > 0 && life.ITInstrument != int(cmd.Instrument)
	wasKeyOff := life.KeyOff
	if sampleChanged && w.Assets.ITCompatibility.mptMixProfile != nil {
		life.KeyOff, life.ITFade = false, false
		life.mptSustainSuppressed = w.Assets.ITCompatibility.mptNoSustainOnPortamento &&
			(len(w.Assets.ITInstruments) == 0 || wasKeyOff)
		life.mptSustainInheritPingPong = w.Assets.Samples[sampleID].LoopPingPong
	} else if instrumentChanged && w.Assets.ITCompatibility.mptNoSustainOnPortamento {
		life.KeyOff, life.ITFade = false, false
		life.mptSustainSuppressed = wasKeyOff || life.mptSustainSuppressed
	}
	retrigger := false
	if sampleID != pb.SampleID {
		cursor.mptMicroTuning = 0
		w.VoicePitches[voice].mptMicroTuning = 0
		oldOPL, newOPL := pb.oplChannel != 0, w.Assets.Samples[sampleID].opl != nil
		if len(w.Assets.ITInstruments) == 0 && w.Assets.ITCompatibility.mptResetFilterOnPorta && !oldOPL && !newOPL {
			life := &w.VoiceLives[voice]
			filter := newITFilter(life.ITFilterCutoff, life.ITFilterResonance, w.Clock.SampleRate,
				true, w.Assets.ITCompatibility.LegacyModPlug, w.Assets.ITCompatibility.ExtendedFilter)
			setITFilterMode(&filter, life.mptHighPass)
			w.VoiceFilters[voice], w.VoiceRightFilters[voice] = filter, filter
		}
		if w.Assets.ITCompatibility.LegacyModPlug || oldOPL != newOPL ||
			(oldOPL && w.Assets.ITCompatibility.oplPortaSampleRetrigger) {
			retrigger = true
			w.stopVoice(voice)
		} else {
			oldSpeed := w.Assets.Samples[pb.SampleID].C5Speed
			newSpeed := w.Assets.Samples[sampleID].C5Speed
			pitch := &w.VoicePitches[voice]
			pitch.BaseFreqHz *= float64(oldSpeed) / float64(newSpeed)
			pitch.ITSlidePitch = 0
			pitch.itFineTuneSpeed = 0
			pb.SampleID, pb.Position, pb.Reverse = sampleID, 0, false
			pb.mptLooped = false
			if newOPL {
				w.opl.setPatch(pb.oplChannel-1, *w.Assets.Samples[sampleID].opl)
			}
		}
	}
	w.VoiceAmps[voice].Volume = float32(w.Assets.Samples[sampleID].Volume) / 64
	if len(w.Assets.ITInstruments) > 0 && !retrigger {
		life := &w.VoiceLives[voice]
		instrument := &w.Assets.ITInstruments[cmd.Instrument-1]
		changed := life.ITInstrument != int(cmd.Instrument)
		life.ITInstrument = int(cmd.Instrument)
		if changed {
			cursor.mptMicroTuning = 0
			w.VoicePitches[voice].mptMicroTuning = 0
			life.mptEnvelopeReleased = false
			life.itPanEnvelopeControl = itEnvelopeDefault
			life.itVolumeEnvelopeControl = itEnvelopeDefault
			life.itPitchEnvelopeControl = itEnvelopeDefault
			life.itHasNewNoteAction = false
			cursor.mptHasNewNoteAction = false
		}
		if w.Assets.ITCompatibility.mptMixProfile != nil && !w.Assets.ITCompatibility.mptNNAReset {
			life.itHasNewNoteAction = false
			cursor.mptHasNewNoteAction = false
		}
		w.VoiceAmps[voice].ITInstrumentVolume = itInstrumentSampleVolume(&w.Assets.Samples[sampleID], instrument)
		if w.Assets.ITCompatibility.CompatibleGxx || w.Assets.ITCompatibility.LegacyModPlug {
			resetCarry := changed || w.Assets.ITCompatibility.mptMixProfile != nil &&
				wasKeyOff && !w.Assets.ITCompatibility.mptCarryAfterNoteOff
			if resetCarry || !instrument.VolumeEnvelope.Carry {
				life.ITEnvelopeTick = 0
				life.mptEnvelopePosition = 0
				life.mptEnvelopeReleased = false
			}
			if resetCarry || !instrument.PanEnvelope.Carry {
				life.ITPanEnvelopeTick = 0
			}
			if resetCarry || !instrument.PitchEnvelope.Carry {
				life.ITPitchEnvelopeTick = 0
			}
		}
	}
	return retrigger
}
