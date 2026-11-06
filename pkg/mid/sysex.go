package mid

// SynthMode is the MIDI dialect announced by the file, as libADLMIDI's m_synthMode.
type SynthMode uint8

const (
	ModeGM SynthMode = 0
	ModeGS SynthMode = 1
	ModeXG SynthMode = 2
)

const (
	vendorRoland       = 0x41
	vendorYamaha       = 0x43
	vendorUniversalNRT = 0x7E
	vendorUniversalRT  = 0x7F

	rolandSend  = 0x12
	rolandModel = 0x42 // GS
	yamahaModel = 0x4C // XG

	masterVolumeDefault = 127
)

// gsDrumChannels maps the channel nibble of a GS percussion address to a MIDI channel.
var gsDrumChannels = [16]uint8{9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 15}

// SysEx handles a system exclusive message. data is the event payload, which
// follows the F0 status and ends with F7. It reports whether it was understood.
func (m *MIDIDecoder) SysEx(dev uint32, data []byte) bool {
	if len(data) < 3 || data[len(data)-1] != 0xF7 {
		return false
	}

	vendor, id := data[0], data[1]
	data = data[2 : len(data)-1]

	switch vendor {
	case vendorUniversalNRT, vendorUniversalRT:
		return m.universalSysEx(id, vendor == vendorUniversalRT, data)
	case vendorRoland:
		return m.rolandSysEx(dev, id, data)
	case vendorYamaha:
		return m.yamahaSysEx(id, data)
	}

	return false
}

func (m *MIDIDecoder) universalSysEx(id byte, realtime bool, data []byte) bool {
	if len(data) < 2 || id != 0x7F && id != m.devID {
		return false
	}

	addr := uint16(data[0]&0x7F)<<8 | uint16(data[1]&0x7F)
	data = data[2:]

	switch {
	case !realtime && addr == 0x0901: // GM System On
		m.mode = ModeGM
		m.ResetState()

		return true
	case !realtime && addr == 0x0902: // GM System Off
		m.mode = ModeXG
		m.ResetState()

		return true
	case realtime && addr == 0x0401 && len(data) == 2: // Master volume
		volume := uint16(data[0]&0x7F) | uint16(data[1]&0x7F)<<7
		m.OPL.SetMasterVolume(int(volume >> 7))

		for ii := range m.chans {
			m.NoteUpdateAll(uint32(ii), UpdVolume)
		}

		return true
	}

	return false
}

func (m *MIDIDecoder) rolandSysEx(dev uint32, id byte, data []byte) bool {
	if len(data) < 6 || id != 0x7F && id&0x0F != m.devID {
		return false
	}

	model, mode := data[0]&0x7F, data[1]&0x7F
	sum := data[len(data)-1] & 0x7F
	data = data[2 : len(data)-1]

	var check byte
	for _, b := range data {
		check += b & 0x7F
	}

	if (128-check&127)&127 != sum {
		return false
	}

	addr := uint32(data[0]&0x7F)<<16 | uint32(data[1]&0x7F)<<8 | uint32(data[2]&0x7F)

	var target uint8

	if addr&0xFFF0FF == 0x401015 { // Turn a channel into percussion
		addr = 0x401015
		target = data[1] & 0x0F
	}

	data = data[3:]

	if mode != rolandSend || model != rolandModel {
		return false
	}

	switch addr {
	case 0x00007F, 0x40007F: // System mode set, mode set
		if len(data) != 1 || id&0xF0 != 0x10 {
			break
		}

		m.mode = ModeGS
		m.ResetState()

		return true
	case 0x401015: // Percussion channel
		if len(data) != 1 || id&0xF0 != 0x10 {
			break
		}

		value := data[0] & 0x7F
		m.chans[dev+uint32(gsDrumChannels[target])].Percussion = value == 0x01 || value == 0x02

		return true
	}

	return false
}

func (m *MIDIDecoder) yamahaSysEx(id byte, data []byte) bool {
	if len(data) < 1 || id != 0x7F && id&0x0F != m.devID {
		return false
	}

	model := data[0] & 0x7F
	data = data[1:]

	if model != yamahaModel || id&0xF0 != 0x10 || len(data) < 3 { // parameter change
		return false
	}

	address := uint32(data[0]&0x7F)<<16 | uint32(data[1]&0x7F)<<8 | uint32(data[2]&0x7F)
	data = data[3:]

	if address == 0x00007E && len(data) == 1 { // XG System On
		m.mode = ModeXG
		m.ResetState()

		return true
	}

	return false
}

// ResetState puts every channel back to its power-on state and silences the
// notes, then restores the master volume.
func (m *MIDIDecoder) ResetState() {
	for ii := range m.chans {
		ch := &m.chans[ii]

		ch.Reset()

		ch.LastLRPN, ch.LastMRPN, ch.NRPN = 0, 0, false

		if m.mode == ModeGS { // reset custom drum channels
			ch.Percussion = false
		}

		m.NoteUpdateAll(uint32(ii), UpdAll)
		m.NoteUpdateAll(uint32(ii), UpdOff)
	}

	m.OPL.SetMasterVolume(masterVolumeDefault)
}
