// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

// Instrument-only changes reuse the last pattern note, not a mapped note.
func (w *Tracker) switchITInstrumentOnly(track TrackID, cmd ModCommand, previousInstrument uint8) {
	if w.Assets.ITCompatibility.mptMixProfile != nil && w.TrackMemories[track].itSpecialNote &&
		(len(w.Assets.ITInstruments) != 0 || w.Assets.ITCompatibility.mptInstrumentWithoutNote) {
		voice := w.TrackCursors[track].ActiveVoice
		if !w.Assets.ITCompatibility.mptInstrumentWithoutNote &&
			len(w.Assets.ITInstruments) != 0 && voice != -1 && w.VoiceLives[voice].Active {
			w.VoiceAmps[voice].Volume = 0
		}
		return
	}
	note := w.TrackMemories[track].LastNote
	if note == 0 {
		return
	}
	sampleID, mappedNote, instrumentID, ok := resolveSampleNote(&w.Assets, cmd.Instrument, note)
	if !ok {
		return
	}
	cursor := &w.TrackCursors[track]
	voice := cursor.ActiveVoice
	active := voice != -1 && w.VoiceLives[voice].Active
	if w.Assets.ITCompatibility.mptInstrumentWithoutNote {
		changed := false
		if active {
			if instrumentID != 0 {
				changed = w.VoiceLives[voice].ITInstrument != instrumentID
			} else {
				changed = w.VoiceSamples[voice].SampleID != sampleID
			}
		} else {
			changed = previousInstrument != 0 && cmd.Instrument != previousInstrument ||
				w.Assets.ITCompatibility.mptMultiSampleInstrument
		}
		if changed && (!active || !itHasTonePortamento(cmd) ||
			(!w.Assets.ITCompatibility.CompatibleGxx || !w.Assets.ITCompatibility.mptPortamentoInstrument) &&
				sampleID != w.VoiceSamples[voice].SampleID) {
			cmd.Note = note
			// A synthesized note does not make the row's offset apply to the restart.
			if cmd.EffectCmd == 0x12 {
				cmd.EffectCmd = 0
			}
			if active && itHasTonePortamento(cmd) {
				// OpenMPT checks NNA before it turns a sample-swapping portamento into a restart.
				if w.startSampleVoiceWithNNA(track, cmd, sampleID, mappedNote, instrumentID, false) {
					w.TrackMemories[track].ITNoteCut = false
				}
			} else {
				w.startVoiceAfterITCut(track, cmd, sampleID, mappedNote, instrumentID)
			}
			return
		}
	}
	previousSample := -1
	if active {
		previousSample = w.VoiceSamples[voice].SampleID
	} else if sample, _, _, valid := resolveSampleNote(&w.Assets, previousInstrument, note); valid {
		previousSample = sample
	}
	if previousSample < 0 {
		if w.Assets.ITCompatibility.mptInitialNoteMemory && previousInstrument == 0 && !w.TrackMemories[track].itSpecialNote {
			cmd.Note = note
			w.startSampleVoice(track, cmd, sampleID, mappedNote, instrumentID)
		}
		return
	}
	if voice == -1 || !w.VoiceLives[voice].Active {
		cmd.Note = note
		w.startSampleVoice(track, cmd, sampleID, mappedNote, instrumentID)
		return
	}
	if w.Assets.ITCompatibility.mptMixProfile != nil && instrumentID != 0 &&
		(!w.Assets.ITCompatibility.mptNNAReset || w.VoiceLives[voice].ITInstrument != instrumentID) {
		cursor.mptHasNewNoteAction = false
		w.VoiceLives[voice].itHasNewNoteAction = false
	}
	if w.Assets.ITCompatibility.mptMixProfile != nil && itHasTonePortamento(cmd) &&
		w.Assets.ITCompatibility.CompatibleGxx && w.Assets.ITCompatibility.mptPortamentoInstrument {
		sampleID = previousSample
	}
	cursor.mptMicroTuning = 0
	w.VoicePitches[voice].mptMicroTuning = 0
	if w.Assets.ITCompatibility.mptMultiSampleInstrument && cmd.Instrument == previousInstrument &&
		sampleID != previousSample {
		w.VoiceAmps[voice].Volume = float32(w.Assets.Samples[sampleID].Volume) / 64
		return
	}
	oldOPL := w.Assets.Samples[previousSample].opl != nil
	newOPL := w.Assets.Samples[sampleID].opl != nil
	if oldOPL != newOPL {
		w.stopVoice(voice)
		cmd.Note = note
		w.startSampleVoice(track, cmd, sampleID, mappedNote, instrumentID)
		return
	}
	playback := &w.VoiceSamples[voice]
	if sampleID != previousSample {
		pitch := &w.VoicePitches[voice]
		pitch.BaseFreqHz *= float64(w.Assets.Samples[previousSample].C5Speed) / float64(w.Assets.Samples[sampleID].C5Speed)
		pitch.ITSlidePitch = 0
		pitch.itFineTuneSpeed = 0
		playback.SampleID = sampleID
		playback.Ended = false
		playback.mptLooped = false
		if newOPL {
			w.opl.setPatch(playback.oplChannel-1, *w.Assets.Samples[sampleID].opl)
		}
	}
	amp := &w.VoiceAmps[voice]
	amp.Volume = float32(w.Assets.Samples[sampleID].Volume) / 64
	if instrumentID != 0 {
		life := &w.VoiceLives[voice]
		life.ITInstrument = instrumentID
		amp.ITInstrumentVolume = itInstrumentSampleVolume(&w.Assets.Samples[sampleID], &w.Assets.ITInstruments[instrumentID-1])
		if w.Assets.ITCompatibility.mptMixProfile != nil && !itHasTonePortamento(cmd) {
			life.KeyOff, life.ITFade = false, false
			life.mptSustainSuppressed = false
			life.mptSustainInheritPingPong = w.Assets.Samples[sampleID].LoopPingPong
			sample := &w.Assets.Samples[sampleID]
			end := len(sample.Data)
			if sample.mptSustainLoop {
				end = min(end, int(sample.mptSustainEnd))
			} else if sample.LoopEnable {
				end = min(end, int(sample.LoopEnd))
			}
			if playback.Position >= float64(end) {
				playback.Position, playback.Reverse = 0, false
				playback.mptLooped = false
			}
		}
	}
}
