// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.
// For OpenMPT attribution and dependency licenses, see NOTICE.

package mod

import (
	"math"

	"github.com/erik-adelbert/duh/pkg/opl3"
)

const oplChannels = 18

type oplPatch [12]uint8

type oplSynth struct {
	chip             *opl3.Chip
	patches          [oplChannels]oplPatch
	keyOn            [oplChannels]uint8
	keyOffFrames     [oplChannels]int
	pendingKeyOn     [oplChannels]bool
	keyOffFrameCount int
}

func newOPLSynth(sampleRate int) *oplSynth {
	if sampleRate <= 0 {
		sampleRate = opl3.YMF262SampleRate
	}

	s := &oplSynth{
		chip:             opl3.NewChip(sampleRate),
		keyOffFrameCount: (sampleRate+opl3.YMF262SampleRate-1)/opl3.YMF262SampleRate + 1,
	}

	s.chip.WriteReg(0x105, 1)
	s.chip.WriteReg(0x104, 0)

	return s
}

func oplChannelRegister(channel int) uint16 {
	if channel >= 9 {
		return uint16(channel-9) | 0x100
	}

	return uint16(channel)
}

func oplOperatorRegister(channel int) uint16 {
	offsets := [9]uint16{0, 1, 2, 8, 9, 10, 16, 17, 18}

	reg := offsets[channel%9]
	if channel >= 9 {
		reg |= 0x100
	}

	return reg
}

func (s *oplSynth) setPatch(channel int, patch oplPatch) {
	s.patches[channel] = patch

	mod := oplOperatorRegister(channel)
	for op := range 2 {
		reg := mod + uint16(op*3)

		for i, base := range [...]uint16{0x20, 0x40, 0x60, 0x80, 0xe0} {
			s.chip.WriteReg(base|reg, patch[2*i+op])
		}
	}
	s.chip.WriteReg(0xc0|oplChannelRegister(channel), patch[10])
}

func (s *oplSynth) setFrequency(channel int, hz float64, on bool) {
	block := 0
	for block < 7 && hz > 48503.0/1000*math.Exp2(float64(block)) {
		block++
	}

	fnum := min(1023, int(math.Round(hz*math.Exp2(float64(20-block))/49716)))
	fnum = max(0, fnum)

	value := uint8((fnum >> 8) | (block << 2))
	if on {
		value |= 0x20
	}

	wasOn := s.keyOn[channel]&0x20 != 0
	s.keyOn[channel] = value

	reg := oplChannelRegister(channel)
	s.chip.WriteReg(0xa0|reg, uint8(fnum))

	// The chip must process a key-off before a subsequent key-on can retrigger
	// its envelope. Both writes can otherwise occur before the next chip sample.
	if on && s.keyOffFrames[channel] > 0 {
		s.pendingKeyOn[channel] = true
		value &^= 0x20
	} else if !on {
		if wasOn {
			s.keyOffFrames[channel] = s.keyOffFrameCount
		}
		s.pendingKeyOn[channel] = false
	}
	s.chip.WriteReg(0xb0|reg, value)
}

func (s *oplSynth) noteOff(channel int) {
	if s.keyOn[channel]&0x20 != 0 {
		s.keyOffFrames[channel] = s.keyOffFrameCount
	}
	s.pendingKeyOn[channel] = false
	s.keyOn[channel] &^= 0x20
	s.chip.WriteReg(0xb0|oplChannelRegister(channel), s.keyOn[channel])
}

func oplLevel(volume, level uint8) uint8 {
	if volume >= 63 {
		return level
	}
	if volume > 0 {
		volume++
	}
	return level&0xc0 | uint8(63-((63-int(level&0x3f))*int(volume))/64)
}

func (s *oplSynth) setVolume(channel int, volume uint8) {
	patch := s.patches[channel]
	reg := oplOperatorRegister(channel)
	if patch[10]&1 != 0 {
		s.chip.WriteReg(0x40|reg, oplLevel(volume, patch[2]))
	}
	s.chip.WriteReg(0x40|(reg+3), oplLevel(volume, patch[3]))
}

func (s *oplSynth) setModulatorCutoff(channel int, cutoff uint8) {
	s.chip.WriteReg(0x40|oplOperatorRegister(channel), oplLevel(cutoff, s.patches[channel][2]))
}

func (s *oplSynth) setPanning(channel int, pan int) {
	value := s.patches[channel][10] &^ 0x30
	if pan <= 170 {
		value |= 0x10
	}
	if pan >= 85 {
		value |= 0x20
	}
	s.chip.WriteReg(0xc0|oplChannelRegister(channel), value)
}

func (s *oplSynth) render(stereo []int16) {
	for len(stereo) > 0 {
		frames := len(stereo) / 2
		for channel, pending := range s.pendingKeyOn {
			if pending {
				frames = min(frames, s.keyOffFrames[channel])
			}
		}

		s.chip.Stream2Ch(stereo[:frames*2], frames*2)

		stereo = stereo[frames*2:]
		for channel, remaining := range s.keyOffFrames {
			if remaining == 0 {
				continue
			}

			s.keyOffFrames[channel] = max(0, remaining-frames)
			if s.keyOffFrames[channel] == 0 && s.pendingKeyOn[channel] {
				s.pendingKeyOn[channel] = false
				s.chip.WriteReg(0xb0|oplChannelRegister(channel), s.keyOn[channel])
			}
		}
	}
}
