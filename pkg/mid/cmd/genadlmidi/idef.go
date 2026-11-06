package main

import mid "github.com/erik-adelbert/duh/pkg/mid"

type idef [11]byte

func (i idef) amvib(op int) byte {
	if op == 1 {
		return i[0]
	}

	return i[1]
}

func (i idef) atdec(op int) byte {
	if op == 1 {
		return i[2]
	}

	return i[3]
}

func (i idef) susrel(op int) byte {
	if op == 1 {
		return i[4]
	}

	return i[5]
}

func (i idef) wave(op int) byte {
	if op == 1 {
		return i[6]
	}

	return i[7]
}

func (i idef) ksltl(op int) byte {
	if op == 1 {
		return i[8]
	}

	return i[9]
}

func (i idef) fb(_ int) byte {
	return i[10]
}

func toOPs(d idef, ops []mid.OP, off int) {
	for i := range 2 {
		ops[off+i].E862 = uint32(d.wave(i))<<24 | uint32(d.susrel(i))<<16
		ops[off+i].E862 |= uint32(d.atdec(i))<<8 | uint32(d.amvib(i))
		ops[off+i].D40 = uint32(d.ksltl(i))
	}
}
