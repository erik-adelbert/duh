// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

func (w *Tracker) allocVoice() VoiceID {
	for i := range MaxVoices {
		if !w.VoiceLives[i].Active {
			return VoiceID(i)
		}
	}
	if !w.Assets.IT {
		return -1
	}

	selected := VoiceID(-1)
	var bestPriority int64
	bestEnvelope := -1
	for i := range MaxVoices {
		life := &w.VoiceLives[i]
		track := int(life.ParentTrack)
		if !life.ITPastNote && track >= 0 && track < len(w.TrackCursors) && w.TrackCursors[track].ActiveVoice == VoiceID(i) {
			continue
		}
		sampleID := w.VoiceSamples[i].SampleID
		if sampleID < 0 || sampleID >= len(w.Assets.Samples) {
			w.stopVoice(VoiceID(i))
			return VoiceID(i)
		}
		priority := w.itBackgroundVoicePriority(i, sampleID)
		if selected == -1 || priority < bestPriority || priority == bestPriority && life.ITEnvelopeTick > bestEnvelope {
			selected, bestPriority, bestEnvelope = VoiceID(i), priority, life.ITEnvelopeTick
		}
	}
	if selected != -1 {
		w.stopVoice(selected)
	}
	return selected
}

func (w *Tracker) itBackgroundVoicePriority(voice, sampleID int) int64 {
	// Approximate OpenMPT's real-volume and note-volume priority, favoring looped voices for reclamation.
	priority := int64(clamp32(voiceVolume(w, voice, sampleID), 0, 1)*16384)*512 +
		int64(clamp32(w.VoiceAmps[voice].Volume, 0, 1)*256)
	sample := &w.Assets.Samples[sampleID]
	if sample.mptSustainLoop && !w.VoiceLives[voice].KeyOff && !w.VoiceLives[voice].mptSustainSuppressed || sample.LoopEnable {
		priority /= 2
	}
	return priority
}
