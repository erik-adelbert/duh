package mod

func (w *Tracker) applyITNoteCut(track TrackID, voice VoiceID, arg uint8, cmd ModCommand) {
	if arg&0xf0 != 0xc0 || w.Clock.Tick != max(1, int(arg&15)) || voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	if !w.Assets.ITCompatibility.LegacyModPlug && cmd.Note >= 1 && cmd.Note <= 120 && itHasVolumeTonePortamento(cmd) {
		return
	}

	w.TrackMemories[track].ITNoteCut = true

	if w.Assets.ITCompatibility.LegacyModPlug ||
		w.Assets.ITCompatibility.mptNoteCutWithPorta && !w.Assets.ITCompatibility.mptCutStopsSample {
		w.VoiceAmps[voice].Volume = 0
	} else {
		w.stopVoice(voice)
	}
}

func (w *Tracker) restoreITCutSample(track TrackID, cmd ModCommand) {
	mem := &w.TrackMemories[track]
	if w.Assets.IT && len(w.Assets.ITInstruments) == 0 && cmd.Note == 0 &&
		w.TrackCursors[track].ActiveVoice == -1 && mem.ITNoteCut {
		note := mem.LastNote
		if note >= 1 && note <= 120 {
			sampleID, mappedNote, instrumentID, ok := resolveSampleNote(&w.Assets, cmd.Instrument, note)
			if ok && w.startSampleVoice(track, cmd, sampleID, mappedNote, instrumentID) {
				mem.ITNoteCut = false
			}
		}
	}

	if w.Assets.ITCompatibility.LegacyModPlug {
		mem.ITNoteCut = false
	}
}

func (w *Tracker) startVoiceAfterITCut(track TrackID, cmd ModCommand, sampleID int, note uint8, instrumentID int) bool {
	cursor := &w.TrackCursors[track]
	mem := &w.TrackMemories[track]
	inheritVolume := w.Assets.IT && w.Assets.ITCompatibility.LegacyModPlug &&
		mem.ITNoteCut && cmd.Instrument == 0 && cursor.ActiveVoice != -1 && w.VoiceLives[cursor.ActiveVoice].Active

	var volume float32
	if inheritVolume {
		volume = w.VoiceAmps[cursor.ActiveVoice].Volume
	}

	if !w.startSampleVoice(track, cmd, sampleID, note, instrumentID) {
		return false
	}

	if inheritVolume {
		w.VoiceAmps[cursor.ActiveVoice].Volume = volume
	} else {
		mem.ITNoteCut = false
	}

	return true
}
