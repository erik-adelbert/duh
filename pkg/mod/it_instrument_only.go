package mod

func (w *Tracker) applyITInstrumentOnly(track TrackID, cmd ModCommand, previousInstrument uint8) {
	mem := &w.TrackMemories[track]

	if w.Assets.ITCompatibility.LegacyModPlug {
		if mem.LastNote == 0 {
			mem.LastInstrument = previousInstrument
			return
		}
		w.switchITInstrumentOnly(track, cmd, previousInstrument)
		return
	}

	if mem.itSpecialNote {
		return
	}

	note := mem.LastNote
	if note == 0 {
		note = 61
	}

	cursor := &w.TrackCursors[track]
	if cursor.ActiveVoice != -1 && w.VoiceLives[cursor.ActiveVoice].Active &&
		w.VoiceLives[cursor.ActiveVoice].ITInstrument == int(cmd.Instrument) {
		sampleID, _, _, ok := resolveSampleNote(&w.Assets, cmd.Instrument, note)
		if ok {
			w.VoiceAmps[cursor.ActiveVoice].Volume = float32(w.Assets.Samples[sampleID].Volume) / 64
		}
		return
	}

	sampleID, mappedNote, instrumentID, ok := resolveSampleNote(&w.Assets, cmd.Instrument, note)
	if !ok {
		return
	}

	cmd.Note = note
	w.startSampleVoice(track, cmd, sampleID, mappedNote, instrumentID)
}
