// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package mid

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

const (
	decoderChunkFrames  = PCMRate / 40
	decoderMaxInputSize = 10 * MB
)

// DecoderOptions configures the OPL synth used to decode a MIDI file.
// Loop enables playback using the sequencer's loop markers. Size and Duration
// always describe one non-looping pass including the one-second tail. A
// loopEnd marker ends that pass, as it does for the non-looping player.
type DecoderOptions struct {
	CardCount   int  // Number of OPL cards to use
	FourOps     int  // Number of hardware 4-op channels to use
	Flags       int  // Synthesizer flags
	Bank        int  // Built-in bank index to use
	VolumeModel int  // Volume model to use
	Loop        bool // Enable looping based on sequencer loop markers
}

// Decoder synthesizes a MIDI file as signed 16-bit little-endian stereo PCM.
// Seeking rebuilds the synthesizer and renders from the beginning.
type Decoder struct {
	data []byte
	opts DecoderOptions

	player *MIDIPlayer

	size      int64
	offset    int64
	frames    int64
	lastDT    time.Duration
	remainder int64
	tail      int
	ending    bool
	closed    bool

	buf     []byte
	pcm     []byte
	samples []int16
	mix     []int32
}

var _ io.ReadSeekCloser = (*Decoder)(nil)
var _ io.ReaderAt = (*Decoder)(nil)

// Decode buffers in and prepares a decoder. With no options, one OPL card,
// no hardware 4-op channels, and built-in bank 0 are used. When options are
// supplied, CardCount must be between 1 and 100.
// Computing the one-pass length scans the sequence before returning.
func Decode(in io.Reader, options ...DecoderOptions) (*Decoder, error) {
	if in == nil {
		return nil, fmt.Errorf("MIDI decode: nil input")
	}

	opts := DecoderOptions{}
	if len(options) > 0 {
		opts = options[0]
	}

	opts.CardCount = clamp(opts.CardCount, 1, 100)
	opts.FourOps = clamp(opts.FourOps, 0, 6*opts.CardCount)
	opts.VolumeModel = clamp(opts.VolumeModel, 0, VolModelCount-1)
	opts.Bank = clamp(opts.Bank, 0, len(banks)-1)

	if ErrBuiltin != nil {
		return nil, mkError(ErrDecode, ErrBuiltin)
	}

	in = io.LimitReader(in, decoderMaxInputSize) // prevent reading more than 10 MB

	data, err := io.ReadAll(in)
	if err != nil {
		return nil, mkError(ErrDecode, err)
	}

	d := new(Decoder{
		data: data,
		opts: opts,
	})

	if err := d.reset(false); err != nil {
		return nil, mkError(ErrDecode, err)
	}

	for {
		n, err := d.nextFrames()

		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if d.frames > math.MaxInt64/4-int64(n) {
			return nil, mkError(ErrDecode, "output size overflow")
		}
		d.frames += int64(n)
	}

	d.size = d.frames * 4
	if err := d.reset(opts.Loop); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Decoder) reset(loop bool) error {

	synth := newSynth(d.opts.CardCount, d.opts.FourOps, d.opts.Flags, uint32(d.opts.Bank))

	p := &MIDIPlayer{Synth: synth, Sequencer: NewSequencer(synth)}
	if err := p.SetBank(d.opts.Bank); err != nil {
		return mkError(ErrDecode, err)
	}
	p.SetVolumeModel(d.opts.VolumeModel)
	p.DeviceSelect("")
	p.SetNoLooping(!loop)

	if err := p.openReader(bytes.NewReader(d.data)); err != nil {
		return err
	}

	d.player = p
	d.offset, d.frames, d.remainder, d.lastDT = 0, 0, PCMRate/2, 0
	d.tail, d.ending = 0, false
	d.buf = nil
	return nil
}

func (d *Decoder) nextFrames() (int, error) {
	if !d.ending {
		wait := d.player.Tick(d.lastDT)
		d.lastDT = 0
		if d.player.IsFinished() {
			d.ending = true
			d.tail = PCMRate
		} else {
			if wait <= 0 {
				return 0, fmt.Errorf("sequencer: no timing progress")
			}

			n := decoderChunkFrames
			if wait < time.Second/40 {
				n = int((int64(wait)*PCMRate + int64(time.Second) - 1) / int64(time.Second))
			}
			d.setElapsed(n)

			return n, nil
		}
	}
	if d.tail == 0 {
		return 0, io.EOF
	}
	n := min(d.tail, decoderChunkFrames)
	d.tail -= n
	d.player.Advance(d.lastDT)
	d.setElapsed(n)
	return n, nil
}

func (d *Decoder) setElapsed(frames int) {
	ns := int64(frames)*int64(time.Second) + d.remainder
	d.lastDT = time.Duration(ns / PCMRate)
	d.remainder = ns % PCMRate
}

// Read reads interleaved stereo PCM bytes. Arbitrary buffer sizes are accepted.
func (d *Decoder) Read(p []byte) (int, error) {
	if d.closed {
		return 0, os.ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	n := 0
	for n < len(p) {
		if len(d.buf) == 0 {
			nframe, err := d.nextFrames()
			if err != nil {
				if n != 0 && err == io.EOF {
					return n, nil
				}
				return n, err
			}

			nsample := nframe * 2
			if cap(d.samples) < nsample {
				d.samples = make([]int16, nsample)
				d.mix = make([]int32, nsample)
			}

			samples, mix := d.samples[:nsample], d.mix[:nsample]
			clear(mix)

			for _, card := range d.player.OPL.Cards {
				card.Stream2Ch(samples, nsample)
				for i, sample := range samples {
					mix[i] += int32(sample)
				}
			}

			if cap(d.pcm) < nsample*2 {
				d.pcm = make([]byte, nsample*2)
			} else {
				d.pcm = d.pcm[:nsample*2]
			}

			d.buf = d.pcm

			for i, sample := range mix {
				sample16 := uint16(int16(clamp(sample, -32768, 32767)))
				binary.LittleEndian.PutUint16(d.buf[i*2:], sample16)
			}

			d.frames += int64(nframe)
		}

		ncopy := copy(p[n:], d.buf)
		d.buf = d.buf[ncopy:]
		n += ncopy
		d.offset += int64(ncopy)
	}

	return n, nil
}

// Seek positions within the first-pass byte range. When looping, the decoder
// follows loop markers rather than rendering the non-looping tail. Seeking is
// implemented by re-rendering and can be slow for long files.
func (d *Decoder) Seek(offset int64, whence int) (int64, error) {
	if d.closed {
		return 0, os.ErrClosed
	}

	base := int64(0)

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = d.offset
	case io.SeekEnd:
		base = d.size
	default:
		return d.offset, mkError(ErrSeek, "invalid whence")
	}

	if offset < -base || offset > d.size-base {
		return d.offset, mkError(ErrSeek, "invalid offset")
	}

	dst := base + offset
	if dst < d.offset {
		if err := d.reset(d.opts.Loop); err != nil {
			return 0, err
		}
	}

	_, err := io.CopyN(io.Discard, d, dst-d.offset)
	if err != nil {
		return d.offset, fmt.Errorf("MIDI seek: %w", err)
	}

	return d.offset, nil
}

// ReadAt reads from the one-pass PCM output without changing the read position.
func (d *Decoder) ReadAt(p []byte, off int64) (int, error) {
	if d.closed {
		return 0, os.ErrClosed
	}
	if off < 0 || off > d.size {
		return 0, fmt.Errorf("MIDI read at: offset outside one-pass output")
	}

	copyDecoder := &Decoder{data: d.data, opts: d.opts, size: d.size}
	if err := copyDecoder.reset(false); err != nil {
		return 0, err
	}

	if _, err := copyDecoder.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}

	n, err := io.ReadFull(copyDecoder, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}

	return n, err
}

// Reset seeks to the beginning of the decoded audio.
func (d *Decoder) Reset() error {
	_, err := d.Seek(0, io.SeekStart)
	return err
}

// Format reports the PCM output format.
func (d *Decoder) Format() pcm.Format {
	f, _ := pcm.NewFormat('s', 16, 2, PCMRate)
	return f
}

// Size returns the byte length of one non-looping pass and its tail.
func (d *Decoder) Size() int64 { return d.size }

// Duration returns the duration of one non-looping pass and its tail.
func (d *Decoder) Duration() time.Duration {
	frames := d.size / 4
	return time.Duration(frames/PCMRate)*time.Second +
		time.Duration(frames%PCMRate)*time.Second/PCMRate
}

// Close releases the decoder. It does not close the supplied reader.
func (d *Decoder) Close() error {
	d.closed = true
	d.buf = nil
	d.pcm = nil
	d.player = nil
	d.data = nil
	d.samples = nil
	d.mix = nil
	return nil
}

func clamp[T ~int32 | ~int](x, min, max T) T {
	switch {
	case x < min:
		return min
	case x > max:
		return max
	}

	return x
}

const MB = 1024 * 1024
