package mod

func (w *Tracker) applyITInstrumentControl(track TrackID, voice VoiceID, arg uint8) {
	if arg <= 0x72 {
		for v := range w.VoiceLives {
			life := &w.VoiceLives[v]
			if !life.Active || !life.ITPastNote || life.ParentTrack != track {
				continue
			}

			switch arg {
			case 0x70:
				w.stopVoice(VoiceID(v))
			case 0x71:
				if life.ITInstrument != 0 {
					releaseITVoice(w, VoiceID(v))
				} else {
					life.KeyOff = true
				}
			case 0x72:
				life.ITFade = true
			}
		}

		return
	}

	if arg >= 0x73 && arg <= 0x76 && w.Assets.ITCompatibility.mptMixProfile != nil {
		cursor := &w.TrackCursors[track]
		cursor.mptNewNoteAction, cursor.mptHasNewNoteAction = arg-0x73, true
	}

	if voice == -1 || !w.VoiceLives[voice].Active {
		return
	}

	life := &w.VoiceLives[voice]
	switch arg {
	case 0x73, 0x74, 0x75, 0x76:
		life.itNewNoteAction, life.itHasNewNoteAction = arg-0x73, true
	case 0x77:
		life.itVolumeEnvelopeControl = itEnvelopeDisabled
		if w.Assets.ITCompatibility.LegacyModPlug ||
			w.Assets.ITCompatibility.mptMixProfile != nil && !w.Assets.ITCompatibility.mptEnvelopePositionHandling ||
			life.ITInstrument == 0 || !w.Assets.ITInstruments[life.ITInstrument-1].VolumeEnvelope.Enabled {
			life.ITEnvelopeVolume = 1
		}
	case 0x78:
		life.itVolumeEnvelopeControl = itEnvelopeEnabledOverride
	case 0x79:
		life.itPanEnvelopeControl = itEnvelopeDisabled
		if w.Assets.ITCompatibility.LegacyModPlug ||
			w.Assets.ITCompatibility.mptMixProfile != nil && !w.Assets.ITCompatibility.mptEnvelopePositionHandling ||
			life.ITInstrument == 0 || !w.Assets.ITInstruments[life.ITInstrument-1].PanEnvelope.Enabled {
			life.mptPanEnvelopeChanged = life.ITPanEnvelopeValue != 0 && w.Assets.ITCompatibility.mptMixProfile != nil
			life.ITPanEnvelopeValue = 0
		}
	case 0x7a:
		life.itPanEnvelopeControl = itEnvelopeEnabledOverride
	case 0x7b:
		life.itPitchEnvelopeControl = itEnvelopeDisabled
		if w.Assets.ITCompatibility.LegacyModPlug ||
			w.Assets.ITCompatibility.mptMixProfile != nil && !w.Assets.ITCompatibility.mptEnvelopePositionHandling ||
			life.ITInstrument == 0 || !w.Assets.ITInstruments[life.ITInstrument-1].PitchEnvelope.Enabled {
			life.ITPitchEnvelopeValue = 0
		}
	case 0x7c:
		life.itPitchEnvelopeControl = itEnvelopeEnabledOverride
	}
}
