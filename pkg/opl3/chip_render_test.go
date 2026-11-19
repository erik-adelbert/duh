// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package opl3

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
)

func renderChipScenario(mode int) [32]byte {
	c := NewChip(48000)
	c.WriteReg(0x105, 1)
	if mode == 1 {
		c.WriteReg(0x104, 0x3f)
	}
	for bank := range 2 {
		for i, op := range opMapping {
			reg := uint16(bank<<8) | uint16(op)
			c.WriteReg(reg+0x20, uint8(1+i%7)|uint8(i%2)<<6|uint8((i/2)%2)<<7)
			c.WriteReg(reg+0x40, uint8(i*3%48))
			c.WriteReg(reg+0x60, 0xf6)
			c.WriteReg(reg+0x80, uint8(0x40+i%12))
			c.WriteReg(reg+0xe0, uint8(i%8))
		}
		for ch := range 9 {
			reg := uint16(bank<<8) | uint16(ch)
			c.WriteReg(reg+0xc0, 0xf0|uint8(ch%4)<<1|uint8(ch%2))
			c.WriteReg(panReg|reg, uint8(ch*13))
			c.WriteReg(reg+0xa0, uint8(70+ch*17))
			c.WriteReg(reg+0xb0, 0x31|uint8(ch%4)<<2)
		}
	}
	h := sha256.New()
	var frame [4]int16
	var pcm [8]byte
	for tick := range 400_000 {
		if tick%8192 == 0 {
			step := tick / 8192
			ch := uint16(step % 9)
			bank := uint16((step / 9) % 2 << 8)
			b0 := uint8(0x11 | (step%4)<<2)
			if step%3 == 0 {
				b0 |= 0x20
			}
			c.QueueReg(bank|0xb0|ch, b0)
			c.QueueReg(bank|0xa0|ch, uint8(25+step*13))
			c.QueueReg(bank|0xc0|ch, uint8(0xf0|(step%8)<<1|step%2))
			c.QueueReg(panReg|bank|ch, uint8(step*11))
		}
		if mode == 2 && tick%16384 == 0 {
			c.QueueReg(0xbd, uint8(0x20|(tick/16384)%32))
		}
		c.Resample4Ch(frame[:])
		for i, sample := range frame {
			binary.LittleEndian.PutUint16(pcm[i*2:], uint16(sample))
		}
		_, _ = h.Write(pcm[:])
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func TestChipRenderingReference(t *testing.T) {
	// Hashes were generated with the renderer before the quiet-operator fast path.
	for _, tt := range []struct {
		name string
		mode int
		want string
	}{
		{"two-op", 0, "ff7db98899779e5e09ff9ef4c9d198fb379025d50be75295c05955b813548329"},
		{"four-op", 1, "7d53a75884894cd5792a2686301311936f8f9cb4bb5abd997b255734330a318e"},
		{"rhythm", 2, "6aff965f18dd19827a5f093885f15df509214dd9420d35a0c06c31f213ec6e55"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf("%x", renderChipScenario(tt.mode))
			if got != tt.want {
				t.Fatalf("PCM hash = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestQuietSlotState(t *testing.T) {
	t.Run("dormant slot retains state", func(t *testing.T) {
		c := NewChip(48000)
		c.WriteReg(0x105, 1)
		s := &c.slots[0]
		s.phout = 17
		s.phrst = true
		s.processInline(c.writeID, 0, false)
		if s.dormantID != c.writeID || s.phout != 17 || !s.phrst {
			t.Fatalf("dormant slot changed state: %+v", s.phaseGen)
		}
	})
	t.Run("quiet slot settles", func(t *testing.T) {
		c := NewChip(48000)
		c.WriteReg(0x105, 1)
		s := &c.slots[0]
		s.egen, s.prout, s.fbmod = envGenAttack, 3, 5
		s.phout, s.phrst = 17, true
		s.processInline(c.writeID, 0, false)
		if s.egen != envGenRelease || s.prout != 0 || s.fbmod != 0 ||
			s.phout != 0 || s.phrst || s.eout != 0x1ff {
			t.Fatalf("quiet slot did not settle: env=%+v phase=%+v", s.envGen, s.phaseGen)
		}
	})
	t.Run("quiet slot advances phase and output", func(t *testing.T) {
		c := NewChip(48000)
		c.WriteReg(0x105, 1)
		s := &c.slots[0]
		s.phase, s.phinc, s.out = 512<<9, 512, 42
		s.processInline(c.writeID, 0, false)
		if s.phase != 513<<9 || s.phout != 512 || s.out != -1 || s.prout != 42 {
			t.Fatalf("quiet slot lost phase or output: phase=%+v out=%d prout=%d", s.phaseGen, s.out, s.prout)
		}
	})
}

func BenchmarkChipRendering(b *testing.B) {
	for _, tt := range []struct {
		name string
		mode int
	}{{"two-op", 0}, {"four-op", 1}, {"rhythm", 2}} {
		b.Run(tt.name, func(b *testing.B) {
			for range b.N {
				renderChipScenario(tt.mode)
			}
		})
	}
}
