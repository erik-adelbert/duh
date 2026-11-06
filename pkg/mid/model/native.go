package model

type Native struct{ Generic }

func (n *Native) Volume(volume *OPLModel) {
	volume.scale(nativeVolume(volume))
}

// nativeVolume is shared by the native and RSXX models.
func nativeVolume(volume *OPLModel) int {
	exp := volume.Get(OPLChanExpression)
	vel := volume.Get(OPLVelocity)
	vol := volume.Get(OPLChanVolume)
	mv := volume.Get(OPLMasterVolume)

	// 4096766 = (127 * 127 * 127) * 2
	return min(vel*vol*exp*mv/4_096_766, 63)
}
