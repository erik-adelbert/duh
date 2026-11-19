package mod

func applyITGlobalVolumeSlide(w *Tracker, track TrackID, arg uint8) {
	if w.Assets.ITCompatibility.LegacyModPlug || w.Assets.ITCompatibility.mptGlobalVolumeSlideShared {
		track = 0
	}
	if w.Assets.ITCompatibility.LegacyModPlug {
		arg = itLegacyMixedVolumeSlide(arg, 0xf0)
	}
	mem := &w.TrackMemories[track].itGlobalVolSlide
	if arg != 0 {
		*mem = arg
	} else {
		arg = *mem
	}
	delta := itVolumeSlideDelta(arg, w.itRepeatedRowTick())
	w.ITGlobalVolume = uint8(max(0, min(128, int(w.ITGlobalVolume)+delta)))
}

func itLegacyMixedVolumeSlide(arg, keep uint8) uint8 {
	if arg&15 != 0 && arg&15 != 15 && arg>>4 != 0 && arg>>4 != 15 {
		return arg & keep
	}
	return arg
}

func itVolumeSlideDelta(arg uint8, tick int) int {
	up, down := int(arg>>4), int(arg&15)
	delta := 0
	switch {
	case down == 15 && up != 0:
		if tick == 0 {
			delta = up
		}
	case up == 15 && down != 0:
		if tick == 0 {
			delta = -down
		}
	case tick != 0 && down == 0:
		delta = up
	case tick != 0 && up == 0:
		delta = -down
	}
	return delta
}
