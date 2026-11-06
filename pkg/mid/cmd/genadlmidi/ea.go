package main

import (
	"os"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func LoadEA(banks *mid.BankDB, id int, bi BankInfo) error {
	f, err := os.Open(bi.FileM)

	if err != nil {
		return err
	}

	defer f.Close()

	bid := banks.AddBank(id, bi.Name, mid.BnkHMI)
	melo := mid.NewMidiBank(0, 0)
	drum := mid.NewMidiBank(0, 0)

	var gms = [28]uint32{
		20: 0x245, 0x24F, 0x263, 0x277, 0x281, 0x28B, 0x29F, 0x2A9,
	}

	var raw [10]byte

	for i := range gms[:20] {
		if _, err := f.ReadAt(raw[:1], int64(0x150+i)); err != nil {
			return err
		}
		ins0 := int(raw[0])

		if _, err := f.ReadAt(raw[:2], int64(0x187+2*ins0)); err != nil {
			return err
		}
		off0 := u16(raw[:2])

		gms[i] = uint32(off0)
	}

	// var dat0 [10]byte
	for i := range gms {
		off0 := gms[i]

		if _, err := f.ReadAt(raw[:], int64(off0)); err != nil {
			return err
		}

		var (
			def idef
			ops [4]mid.OP
		)

		def[0] = raw[0] // reg 0x20: modulator AM/VIG/EG/KSR
		def[8] = raw[1] // reg 0x40: modulator ksl/attenuation
		def[2] = raw[2] // reg 0x60: modulator attack/decay
		def[4] = raw[3] // reg 0x80: modulator sustain/release

		def[1] = raw[4] // reg 0x20: carrier AM/VIG/EG/KSR
		def[9] = raw[5] // reg 0x40: carrier ksl/attenuation
		def[3] = raw[6] // reg 0x60: carrier attack/decay
		def[5] = raw[7] // reg 0x80: carrier sustain/release

		def[10] = raw[8] // reg 0xC0 (feedback and connection)

		toOPs(def, ops[:], 0)

		ins := mid.NewInsEntry()
		ins.SetFBConn(uint16(raw[8]), 0)
		ins.Note.Off0 = int8(raw[9] + 12)

		banks.AddIns(melo, i, ins, ops[:], bi.FileM)

		patch := -1
		switch i {
		case 10:
			ins.DrumKey = 0x49
			patch = 0x36
		case 16:
			ins.DrumKey = 0x0C
			patch = 0x24
		case 17:
			ins.DrumKey = 0x01
			patch = 0x26
		case 18:
			ins.DrumKey = 0x17
			patch = 0x2A
		}

		if patch != -1 {
			banks.AddIns(drum, patch, ins, ops[:], bi.FileM)
		}
	}

	banks.AddMidi(bid, melo, false)
	banks.AddMidi(bid, drum, true)

	return nil
}
