package model

type Rsxx struct{ Generic }

func (r *Rsxx) Volume(volume *OPLModel) {
	volume.Set(OPLCarN, volume.Get(OPLCarN)-nativeVolume(volume)/2)
}
