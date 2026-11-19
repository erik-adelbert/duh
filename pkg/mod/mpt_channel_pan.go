// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) applyMPTNoteDefaultPan(cursor *TrackCursor, voice VoiceID, pan float32) {
	amp := &w.VoiceAmps[voice]

	if w.Assets.ITCompatibility.itRestoreChannelPan {
		applyITDefaultPanning(cursor, amp, pan)
	} else {
		cursor.Panning = pan
		cursor.Surround = false
		amp.Panning, amp.Surround = cursor.Panning, false
	}
}

func (w *Tracker) applyMPTPitchPan(cursor *TrackCursor, voice VoiceID, note uint8) {
	instrument := &w.Assets.ITInstruments[w.VoiceLives[voice].ITInstrument-1]
	if instrument.PitchPanSeparation == 0 {
		return
	}

	if !cursor.itHasRestorePan {
		cursor.itRestorePan = cursor.Panning
		cursor.itRestoreSurround = cursor.Surround
		cursor.itHasRestorePan = true
	}

	separation := max(-32, min(32, int(instrument.PitchPanSeparation)))

	cursor.Panning = itPitchPanDepth(
		w.VoiceAmps[voice].Panning, note, instrument.PitchPanCenter, separation,
	)

	w.VoiceAmps[voice].Panning = cursor.Panning
}

func applyITDefaultPanning(cursor *TrackCursor, amp *AmplitudeState, pan float32) {
	cursor.itRestorePan = cursor.Panning
	cursor.itRestoreSurround = cursor.Surround
	cursor.itHasRestorePan = true
	cursor.itHasRestorePan = true
	cursor.Panning = pan
	cursor.Surround = false

	amp.Panning, amp.Surround = cursor.Panning, false
}
