package mid

import "github.com/erik-adelbert/duh/pkg/mid/model"

// NewVolumeModel returns the volume and frequency model for a Vol* id.
// Unknown ids fall back to the generic model.
func NewVolumeModel(id int) model.Volume {
	switch id {
	case VolNative:
		return &model.Native{}
	case VolDMX:
		return &model.Dmx{}
	case VolApogee:
		return &model.Apogee{}
	case VolWin9x:
		return &model.SB16{}
	case VolDMXFixed:
		return &model.DmxFixed{}
	case VolApogeeFixed:
		return &model.ApogeeFixed{}
	case VolAIL:
		return &model.Ail{}
	case VolWin9xFM:
		return &model.Win9x{}
	case VolHMI:
		return &model.Hmi{}
	case VolHMIOld:
		return &model.HmiOld{}
	case VolMSAdlib:
		return &model.Adlib{}
	case VolIMFCreator:
		return &model.ImfCreator{}
	case VolOConnel:
		return &model.OConnel{}
	}

	return &model.Generic{}
}

// BankVolumeModel returns the volume model id declared by a bank's setup.
func BankVolumeModel(bank int) int {
	if bank < 0 || bank >= len(Banks) {
		return VolLinear
	}

	return int(Banks[bank].Setup & 0xFF)
}
