// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opl3

import "testing"

func TestMixRoutingAndCallbacks(t *testing.T) {
	values := [4]int16{30000, 20000, -12345, 20202}
	for _, side := range []mixSide{mixLeft, mixRight} {
		for nout := 1; nout <= 4; nout++ {
			for _, gain := range []uint16{0, 32768, 65535} {
				for _, masks := range [][2]uint16{{0xffff, 0}, {0, 0xffff}, {0xffff, 0xffff}, {0x5555, 0xaaaa}} {
					c := NewChip(48000)
					ch := &c.chans[0]
					ch.nout = uint8(nout)
					ch.chl, ch.chr = gain, gain
					ch.cha, ch.chb, ch.chc, ch.chd = masks[0], masks[0], masks[1], masks[1]
					for i := range values {
						ch.outL[i], ch.outR[i] = &values[i], &values[i]
					}

					if side == mixLeft {
						c.mixes[side] = []mixChannel{{ch, &ch.outL, &ch.chl, &ch.cha, &ch.chc}}
					} else {
						c.mixes[side] = []mixChannel{{ch, &ch.outR, &ch.chr, &ch.chb, &ch.chd}}
					}
					var gotSide int
					var gotID uint8
					var gotSample int32
					var calls int
					c.SetChannelCallback(func(id uint8, side int, sample int32) {
						gotID, gotSide, gotSample = id, side, sample
						calls++
					})
					c.mix(side)

					acc := values[0]
					for i := 1; i < nout; i++ {
						acc += values[i]
					}
					if gain != 0xffff {
						acc = int16(int32(acc) * int32(gain) / 0xffff)
					}
					expected := func(mask uint16) int32 {
						return int32(int16(uint16(acc) & mask))
					}
					if a, b := c.mixBuffer[side], c.mixBuffer[side+2]; a != expected(masks[0]) || b != expected(masks[1]) {
						t.Fatalf("side=%d nout=%d gain=%d masks=%x: outputs %d,%d want %d,%d", side, nout, gain, masks, a, b, expected(masks[0]), expected(masks[1]))
					}
					if calls != 1 || gotID != ch.id || gotSide != int(side) || gotSample != expected(masks[0]|masks[1]) {
						t.Fatalf("side=%d nout=%d gain=%d masks=%x: callback calls=%d id=%d side=%d sample=%d", side, nout, gain, masks, calls, gotID, gotSide, gotSample)
					}
				}
			}
		}
	}
}

func TestMixRoutingUsesCurrentChannelFields(t *testing.T) {
	c := NewChip(48000)
	c.WriteReg(0x105, 1)
	c.WriteReg(0xc0, 0x30)
	c.rebuildMixLists()

	ch := &c.chans[0]
	original, updated := int16(1234), int16(-5678)
	ch.outL[0], ch.outR[0] = &original, &original
	ch.nout = 1
	ch.chl, ch.chr = 0xffff, 0xffff
	ch.cha, ch.chb, ch.chc, ch.chd = 0xffff, 0xffff, 0, 0
	c.mix(mixLeft)
	if c.mixBuffer[0] != int32(original) {
		t.Fatalf("initial output = %d, want %d", c.mixBuffer[0], original)
	}

	ch.outL[0] = &updated
	ch.chl = 32768
	ch.cha, ch.chc = 0, 0xffff
	c.mix(mixLeft)
	want := int32(int16(int32(updated) * int32(ch.chl) / 0xffff))
	if c.mixBuffer[0] != 0 || c.mixBuffer[2] != want {
		t.Fatalf("updated output = %d,%d, want 0,%d", c.mixBuffer[0], c.mixBuffer[2], want)
	}
}
