package mid

import (
	"fmt"
	"time"
)

func (s *Synth) rtChannel(channel uint8, values ...uint8) (uint32, error) {
	if channel >= 16 {
		return 0, fmt.Errorf("MIDI channel %d is outside 0..15", channel)
	}

	for _, value := range values {
		if value > 127 {
			return 0, fmt.Errorf("MIDI data byte %d is outside 0..127", value)
		}
	}

	return s.DeviceSelect("") + uint32(channel), nil
}

// RTNoteOn starts a note and reports whether a voice was allocated.
// Velocity zero releases the note and returns false.
func (s *Synth) RTNoteOn(channel, note, velocity uint8) (bool, error) {
	ch, err := s.rtChannel(channel, note, velocity)
	if err != nil {
		return false, err
	}
	return s.playbackNoteOn(ch, note, velocity), nil
}

// RTNoteOff releases a note.
func (s *Synth) RTNoteOff(channel, note uint8) error {
	ch, err := s.rtChannel(channel, note)
	if err != nil {
		return err
	}
	s.NoteOff(ch, note)
	return nil
}

// RTNoteAfterTouch applies polyphonic pressure to a note.
func (s *Synth) RTNoteAfterTouch(channel, note, pressure uint8) error {
	ch, err := s.rtChannel(channel, note, pressure)
	if err != nil {
		return err
	}
	s.NoteAftertouch(ch, note, pressure)
	return nil
}

// RTChannelAfterTouch applies channel pressure.
func (s *Synth) RTChannelAfterTouch(channel, pressure uint8) error {
	ch, err := s.rtChannel(channel, pressure)
	if err != nil {
		return err
	}
	s.ChannelPressure(ch, pressure)
	return nil
}

// RTControllerChange applies a MIDI control change.
func (s *Synth) RTControllerChange(channel, controller, value uint8) error {
	ch, err := s.rtChannel(channel, controller, value)
	if err != nil {
		return err
	}
	s.Controller(ch, controller, value)
	return nil
}

// RTPatchChange selects a MIDI program.
func (s *Synth) RTPatchChange(channel, patch uint8) error {
	ch, err := s.rtChannel(channel, patch)
	if err != nil {
		return err
	}
	s.ProgramChange(ch, patch)
	return nil
}

// RTPitchBend applies a 14-bit MIDI pitch bend centered at 8192.
func (s *Synth) RTPitchBend(channel uint8, pitch uint16) error {
	if pitch > 0x3fff {
		return fmt.Errorf("MIDI pitch bend %d is outside 0..16383", pitch)
	}
	ch, err := s.rtChannel(channel)
	if err != nil {
		return err
	}
	s.PitchBend(ch, pitch)
	return nil
}

// RTPitchBendML applies a pitch bend from its MIDI MSB and LSB.
func (s *Synth) RTPitchBendML(channel, msb, lsb uint8) error {
	if _, err := s.rtChannel(channel, msb, lsb); err != nil {
		return err
	}
	return s.RTPitchBend(channel, uint16(msb)<<7|uint16(lsb))
}

// RTBankChangeMSB selects the MSB of the MIDI bank, not the ADL bank family.
func (s *Synth) RTBankChangeMSB(channel, msb uint8) error {
	return s.RTControllerChange(channel, 0, msb)
}

// RTBankChangeLSB selects the LSB of the MIDI bank, not the ADL bank family.
func (s *Synth) RTBankChangeLSB(channel, lsb uint8) error {
	return s.RTControllerChange(channel, 32, lsb)
}

// RTBankChange selects the MIDI bank, with MSB in the high byte and LSB in
// the low byte, matching libADLMIDI's bank packing.
func (s *Synth) RTBankChange(channel uint8, bank uint16) error {
	msb, lsb := uint8(bank>>8), uint8(bank)
	ch, err := s.rtChannel(channel, msb, lsb)
	if err != nil {
		return err
	}
	s.Controller(ch, 0, msb)
	s.Controller(ch, 32, lsb)
	return nil
}

// RTSystemExclusive handles a complete F0...F7 message and reports whether
// its contents were understood.
func (s *Synth) RTSystemExclusive(msg []byte) (bool, error) {
	if len(msg) < 4 || msg[0] != 0xf0 || msg[len(msg)-1] != 0xf7 {
		return false, fmt.Errorf("MIDI SysEx must begin with F0 and end with F7")
	}
	base := s.DeviceSelect("")
	return s.SysEx(base, msg[1:]), nil
}

// RTResetState resets channel controllers and releases active and held notes.
func (s *Synth) RTResetState() {
	s.ResetState()
	s.KillSustainingNotes(-1, -1, holdAny)
}

// RTAllNotesOff immediately stops active and sustained notes on all devices.
func (s *Synth) RTAllNotesOff() {
	for ch := range s.mchans {
		s.Controller(uint32(ch), 120, 0)
	}
	s.KillSustainingNotes(-1, -1, holdAny)
}

// RTGenerate fills out with interleaved stereo signed 16-bit samples at
// PCMRate. The length must be even. Multiple OPL cards are mixed with
// saturation. It advances synth effects but does not advance a file
// sequencer; callers must deliver events between renders and must not also
// call Advance for the same elapsed time. The return value is the number of
// samples, not stereo frames.
func (s *Synth) RTGenerate(out []int16) (int, error) {
	if len(out)%2 != 0 {
		return 0, fmt.Errorf("MIDI audio output must contain whole stereo frames")
	}

	const chunkSamples = 512 * 2
	if len(out) == 0 {
		return 0, nil
	}
	if s.rtSamples == nil {
		s.rtSamples = make([]int16, chunkSamples)
		s.rtMix = make([]int64, chunkSamples)
	}

	for start := 0; start < len(out); {
		n := min(chunkSamples, len(out)-start)
		samples, mix := s.rtSamples[:n], s.rtMix[:n]
		clear(mix)

		for _, card := range s.OPL.Cards {
			card.Stream2Ch(samples, n)

			for i, sample := range samples {
				mix[i] += int64(sample)
			}
		}

		for i, sample := range mix {
			out[start+i] = clipRTSample(sample)
		}

		ns := int64(n/2)*int64(time.Second) + s.rtRemainder

		s.Advance(time.Duration(ns / PCMRate))

		s.rtRemainder = ns % PCMRate

		start += n
	}

	return len(out), nil
}

func clipRTSample(sample int64) int16 {
	return int16(max(-32768, min(32767, sample)))
}
