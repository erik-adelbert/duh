package mod

// OPL has 18 two-operator channels. Released voices can be reclaimed; keyed-on
// voices are never stolen, matching OpenMPT's allocation policy.
func (w *Tracker) allocOPLVoice(voice VoiceID) int {
	released := -1
	for channel, owner := range w.oplVoices {
		if owner == -1 {
			w.oplVoices[channel] = voice
			return channel + 1
		}
		if w.VoiceLives[owner].KeyOff {
			released = channel
		}
	}

	if released < 0 {
		return 0
	}

	w.stopVoice(w.oplVoices[released])
	w.oplVoices[released] = voice
	return released + 1
}
