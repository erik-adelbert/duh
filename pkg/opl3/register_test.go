// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opl3

import "testing"

func TestRegistersChannelBothBanksAndOperators(t *testing.T) {
	for id := range ChannelCount {
		bank, local := id/9, id%9
		var regs Registers
		regs[bank][0xa0+local] = 0xa5
		regs[bank][0xb0+local] = 0x20 | (5 << 2) | 2
		regs[bank][0xc0+local] = (6 << 1) | 1
		for i, o := range opMapping[local*2 : local*2+2] {
			regs[bank][0x20+o] = 0xf0 | uint8(i+2)
			regs[bank][0x40+o] = 0x80 | uint8(i+10)
			regs[bank][0x60+o] = 0x34
			regs[bank][0x80+o] = 0x56
			regs[bank][0xe0+o] = uint8(i + 3)
		}
		ch := regs.Channel(id)
		if ch.ID != id {
			t.Fatalf("channel %d: ID %d", id, ch.ID)
		}
		for i, op := range ch.Ops {
			if op.FQ != 0x2a5 || op.OpState != (OpState{uint8(i + 2), 2, uint8(i + 10), 3, 4, 5, 6, 5, 6}) ||
				op.WF != Waveform(i+3) || op.OpFlags != 0x3f {
				t.Fatalf("channel %d operator %d: %+v", id, i, op)
			}
		}
		regs[bank][0xb0+local] &^= 0x20
		op := regs.Channel(id).Ops[0]
		if op.Has(KEY) || !op.Has(AM) || !op.Has(KSR) {
			t.Fatalf("channel %d: operator flags lost when key is off: %v", id, op.OpFlags)
		}
	}
}

func TestRegistersStatus(t *testing.T) {
	for _, tt := range []struct {
		bank, reg, data uint8
		flag            OPL3Flag
	}{
		{0, 0x08, 0x80, CSM}, {0, 0x01, 0x20, WF},
		{0, 0xbd, 0x01, HH}, {0, 0xbd, 0x02, CYM},
		{0, 0xbd, 0x04, TOM}, {0, 0xbd, 0x08, SD},
		{0, 0xbd, 0x10, BD}, {0, 0xbd, 0x20, DRUM},
		{0, 0xbd, 0x40, VIBI}, {0, 0xbd, 0x80, AMI},
		{1, 0x05, 0x01, OPL3}, {1, 0x04, 0x20, FOUROP},
	} {
		var regs Registers
		regs[tt.bank][tt.reg] = tt.data
		if got, want := regs.Status(), Flags(1<<tt.flag); got != want {
			t.Errorf("register %x:%02x = %02x: status %x, want %x", tt.bank, tt.reg, tt.data, got, want)
		}
	}
	var regs Registers
	regs[1][0x04] = 0xc0
	regs[1][0x05] = 0x02
	if got := regs.Status(); got != 0 {
		t.Fatalf("reserved bits set status %x", got)
	}
}
