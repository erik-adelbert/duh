// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import "math"

func itArpeggio(w *Tracker, track TrackID, voice VoiceID, semitones int) {
	pitch := &w.VoicePitches[voice]
	freq := pitch.BaseFreqHz * math.Pow(2, float64(semitones)/12)

	if w.Assets.ITCompatibility.LegacyModPlug && semitones != 0 {
		mem := &w.TrackMemories[track]
		note := mem.LastNote

		if note >= 1 && note <= 120 && len(w.Assets.ITInstruments) > 0 {
			_, mapped, _, ok := resolveSampleNote(
				&w.Assets, mem.LastInstrument, note,
			)
			if ok {
				note = mapped
			}
		}

		if note >= 1 && note <= 120 {
			sample := &w.Assets.Samples[w.VoiceSamples[voice].SampleID]
			freq, _ = itLegacyNotePitch(
				note-1+uint8(semitones),
				sample,
				w.Assets.ITCompatibility.LinearSlides,
			)
		}
	}

	if pitch.itVibratoDelta != 0 {
		tmp := *pitch
		tmp.BaseFreqHz, tmp.ITSlidePitch = freq, 0

		offset, ok := itVibratoOffset(
			tmp,
			&w.Assets.Samples[w.VoiceSamples[voice].SampleID],
			w.Assets.ITCompatibility,
			pitch.itVibratoDelta,
		)

		if !ok {
			w.stopVoice(voice)
			return
		}
		freq += offset
	}
	pitch.VibratoOffset = freq - pitch.BaseFreqHz
}
