package model

type SB16 struct{ Win9x }

func (s *SB16) Volume(volume *OPLModel) {
	exp := volume.Get(OPLChanExpression)
	vol := volume.Get(OPLChanVolume)
	mv := volume.Get(OPLMasterVolume)

	v := (vol * exp * mv) / 16129
	v = sb16Models[v>>2]

	set := func(f OPLField) {
		vel := volume.Get(OPLVelocity)

		fv := volume.Get(f)
		fv += v + sb16Models[vel>>2]
		fv = min(fv, 63)

		volume.Set(f, fv)
	}

	if volume.IsCar() {
		set(OPLCarN)
	}

	if volume.IsMod() {
		set(OPLModN)
	}
}

var sb16Models = [...]int{
	80, 63, 40, 36, 32, 28, 23, 21,
	19, 17, 15, 14, 13, 12, 11, 10,
	9, 8, 7, 6, 5, 5, 4, 4,
	3, 3, 2, 2, 1, 1, 0, 0,
}
