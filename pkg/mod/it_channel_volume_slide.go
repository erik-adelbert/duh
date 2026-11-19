package mod

func applyITChannelVolumeSlide(w *Tracker, track TrackID, arg uint8) {
	if w.Assets.ITCompatibility.LegacyModPlug {
		arg = itLegacyMixedVolumeSlide(arg, 0x0f)
	}

	mem := &w.TrackMemories[track].itChannelVolSlide
	if arg != 0 {
		*mem = arg
	} else {
		arg = *mem
	}

	delta := itVolumeSlideDelta(arg, w.itRepeatedRowTick())
	w.ITChannelVolume[track] = uint8(max(0, min(64, int(w.ITChannelVolume[track])+delta)))
}
