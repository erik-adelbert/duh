// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) applyITDuplicateNotes(track TrackID, sampleID, instrumentID int, realNote, mappedNote uint8) bool {
	if instrumentID == 0 {
		return false
	}
	acrossInstruments := w.Assets.ITCompatibility.LegacyModPlug || w.Assets.ITCompatibility.itDCTSampleAcrossInstruments
	foregroundCut := false
	for i := range w.VoiceLives {
		life := &w.VoiceLives[i]
		if !life.Active || life.ParentTrack != track || life.ITInstrument == 0 {
			continue
		}

		instrument := &w.Assets.ITInstruments[life.ITInstrument-1]
		sameInstrument := life.ITInstrument == instrumentID
		match := false
		switch instrument.DuplicateCheck {
		case 1:
			note, previous := realNote, w.VoicePitches[i].itRealNote

			if w.Assets.ITCompatibility.LegacyModPlug ||
				w.Assets.ITCompatibility.mptMixProfile != nil &&
					(acrossInstruments || !w.Assets.ITCompatibility.mptRealNoteMapping) {
				note, previous = mappedNote, w.VoicePitches[i].Note
			}
			match = sameInstrument && note != 0 && note == previous
		case 2:
			match = w.VoiceSamples[i].SampleID == sampleID && (sameInstrument || acrossInstruments)
		case 3:
			match = sameInstrument
		}
		if !match {
			continue
		}

		switch instrument.DuplicateAction {
		case 0:
			foregroundCut = foregroundCut || w.TrackCursors[track].ActiveVoice == VoiceID(i)
			w.VoiceAmps[i].Volume = 0
			w.stopVoice(VoiceID(i))
		case 1:
			releaseITVoice(w, VoiceID(i))
		case 2:
			life.ITFade = true
		}
	}

	return foregroundCut
}
