package model

type Volume interface {
	Freq(tone float64, offset *uint32) uint16
	Volume(volume *OPLModel)
}
