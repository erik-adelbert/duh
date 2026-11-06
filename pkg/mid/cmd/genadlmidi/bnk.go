package main

import (
	"os"
	"slices"

	mid "github.com/erik-adelbert/duh/pkg/mid"
)

func LoadBNK(banks *mid.BankDB, id int, bi BankInfo) error {
	perc := bi.FileD != ""

	fn := bi.FileM
	if perc {

		fn = bi.FileD
	}

	raw, err := os.ReadFile(fn)

	if err != nil {
		return err
	}

	bid := banks.AddBank(id, bi.Name, mid.BnkHMI)
	mb := mid.NewMidiBank(0, 0)

	if !slices.Contains([]string{"ADLIB-", "AMLIB-", "ANLIB-"}, string(raw[2:8])) {
		return os.ErrInvalid
	}

	// version := u16(raw[0:2])
	nused := u16(raw[8:10])
	// nentry := u16(raw[10:12])
	nameOff := u32(raw[12:16])
	dataOff := u32(raw[16:20])

	for i := range nused {
		off0 := nameOff + uint32(i)*12

		if off0+12 > uint32(len(raw)) {
			return os.ErrInvalid
		}

		dix := u16(raw[off0 : off0+2])
		flag := raw[off0+2]

		off1 := dataOff + uint32(dix)*30
		if off1+30 > uint32(len(raw)) {
			return os.ErrInvalid
		}

		op0 := raw[off1+2 : off1+15]
		op1 := raw[off1+15 : off1+28]
		wvmod := raw[off1+28]
		wvcar := raw[off1+29]

		gmid := int(i)
		if perc {
			gmid += 0x80
		}

		ins := mid.NewInsEntry()

		dbuf := idef{
			0: mkop(op0),
			1: mkop(op1),

			2: op0[3]*0x10 + op0[6],
			3: op1[3]*0x10 + op1[6],

			4: op0[4]*0x10 + op0[7],
			5: op1[4]*0x10 + op1[7],

			6: wvmod,
			7: wvcar,

			8: op0[0]*0x40 + op0[8],
			9: op1[0]*0x40 + op1[8],

			10: op0[2]*2 + op0[12],
		}

		var ops [4]mid.OP
		toOPs(dbuf, ops[:], 0)

		if perc {
			ins.DrumKey = flag
		}

		ins.FBConn = 2*uint16(op0[1]) + uint16(op1[12])

		banks.AddIns(mb, gmid&0x7F, ins, ops[:], fn)
	}

	banks.AddMidi(bid, mb, perc)

	return nil
}

func mkop(op []byte) byte {
	return op[9]<<7 | op[10]<<6 | op[5]<<5 | op[11]<<4 | op[1]
}
