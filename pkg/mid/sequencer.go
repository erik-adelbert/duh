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
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"
)

// MIDIEventReceiver describes the channel and system-exclusive events handled
// by Synth. Channel numbers include the selected device offset.
type MIDIEventReceiver interface {
	NoteOn(uint32, uint8, uint8)
	NoteOff(uint32, uint8)
	NoteAftertouch(uint32, uint8, uint8)
	Controller(uint32, uint8, uint8)
	ProgramChange(uint32, uint8)
	ChannelPressure(uint32, uint8)
	PitchBend(uint32, uint16)
	SysEx(uint32, []byte) bool
	DeviceSelect(string) uint32
}

var _ MIDIEventReceiver = (*Synth)(nil)

type synth interface {
	MIDIEventReceiver
	playbackNoteOn(uint32, uint8, uint8) bool
	playbackNoteOff(uint32, uint8, uint8)
	prepareFilePlayback() error
	setFileFormat(Format)
	addCMFInstrument(adlData)
	useCMFBank()
	useRSXXDefaults()
	writeOPL(int, int)
}

// Sequencer reads music files and schedules their events for a synth.
// Tick does not advance the synth's voice timers; call Synth.Advance separately.
type Sequencer struct {
	curDev map[uint32]uint32

	synth       synth
	pos         struct{ cur, loop position }
	tracks      [][]byte
	invδ, tempo frac
	loop        struct {
		start, end bool
		generation uint64
		tempo      frac
	}
	music Format
	quit  struct{ noloop, flag bool }
}

// NewSequencer connects file playback to an existing, non-nil synth.
func NewSequencer(synth *Synth) *Sequencer {
	if synth == nil {
		panic("mid: NewSequencer requires a non-nil synth")
	}
	return newSequencer(synth)
}

func newSequencer(target synth) *Sequencer {
	return &Sequencer{
		synth:  target,
		curDev: make(map[uint32]uint32),
		tempo:  newFrac(1, 192),
		invδ:   newFrac(1, 1000000*192),
	}
}

// position tracks the playback position
type position struct {
	ID uint16

	isPlaying bool

	wait time.Duration

	// TickWait     uint64
	// Tick0, TickN uint64

	tracks []trackInfo
}

func (p position) clone() position {
	p.tracks = append([]trackInfo(nil), p.tracks...)
	for i := range p.tracks {
		if p.tracks[i].Reader != nil {
			reader := *p.tracks[i].Reader
			p.tracks[i].Reader = &reader
		}
	}
	return p
}

// trackInfo holds info about a single track's position
type trackInfo struct {
	*bytes.Reader
	delay  int64
	status int

	// LastEvent int
	// State0    TrackDump
}

// SetNoLooping controls whether playback stops when the song reaches its end.
func (s *Sequencer) SetNoLooping(noLoop bool) {
	s.quit.noloop = noLoop
}

func (s *Sequencer) Quit() {
	s.quit.flag = true
}

// IsFinished reports whether playback has reached the end in no-loop mode.
func (s *Sequencer) IsFinished() bool {
	return s.quit.flag
}

// ReadDelay reads a delta time. Like libADLMIDI's end-silence skipping, a
// delay that only precedes the end-of-track event is dropped: some files pad
// a track with minutes of silence that would otherwise be played.
func (s *Sequencer) ReadDelay(tk int) int64 {
	delay := int64(s.ReadVar(tk))

	if delay > 0 {
		tr := s.pos.cur.tracks[tk]

		var ev [2]byte

		n, _ := io.ReadFull(tr.Reader, ev[:])
		tr.Seek(-int64(n), io.SeekCurrent)

		if ev == [2]byte{0xFF, 0x2F} {
			return 0
		}
	}

	return delay
}

// ReadVar reads a variable-length quantity from track data
func (s *Sequencer) ReadVar(tk int) (v uint32) {
	tr := s.pos.cur.tracks[tk].Reader

	for {
		x, _ := tr.ReadByte()

		v = (v << 7) + uint32(x&0x7F)

		if x&0x80 == 0 {
			break
		}
	}

	return v
}

func unwrapRIFF(r io.ReadSeeker) error {
	return reskip(r)
}

// reskip walks a RIFF container looking for the first non-container
// chunk (e.g. an "MThd" MIDI chunk inside an RMID file) and leaves r
// positioned at the start of that chunk's data.
//
// It correctly handles:
//   - nested RIFF/LIST containers
//   - chunk padding to even boundaries
//   - arbitrary number of sibling chunks
func reskip(r io.ReadSeeker) error {
	const chunkHdrSz = 8 // id + size

	var hdr [12]byte

	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}

	if string(hdr[0:4]) != "RIFF" {
		return ErrNotRIFF
	}

	chunk := hdr[:8]
	for {
		_, err := io.ReadFull(r, chunk)

		switch {
		case err == io.EOF:
			return io.ErrUnexpectedEOF
		case err != nil:
			return err
		}

		id := string(chunk[0:4])
		size := u32(chunk[4:8])

		switch id {
		case "RIFF", "LIST":
			// Container chunk: its payload begins with a 4-byte form type,
			// then nested chunks. Recurse into it.
			if err := reskip(r); err != nil {
				return err
			}

			return nil // inner walk already positioned us at the real data
		case "data":
			// Common wrapper in RMID: "data" chunk holds the MIDI bytes.
			// Leave r positioned at the start of the data payload.
			return nil
		default:
			// Unknown/irrelevant chunk: skip its payload + padding.
			skip := int64(size)

			if size&1 == 1 {
				skip++ // RIFF pads to even boundaries
			}

			if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
				return err
			}
		}
	}
}

func (s *Sequencer) Open(filename string) (err error) {
	fd, err := os.Open(filename)
	if err != nil {
		return mkError(ErrMidi, err)
	}
	defer fd.Close()

	return s.openReader(fd)
}

func (s *Sequencer) openReader(fd io.ReadSeeker) (err error) {
	defer func() {
		err = mkError(ErrMidi, err)
	}()

	hdr := make([]byte, 14)
	if _, err = io.ReadFull(fd, hdr); err != nil {
		return err
	}

	if string(hdr[:4]) == "RIFF" {
		if err = unwrapRIFF(fd); err != nil {
			return err
		}

		if _, err = io.ReadFull(fd, hdr); err != nil {
			return err
		}
	}

	// Detect format from hdr and dispatch.
	format := s.detectFormat(fd, hdr)

	switch format {
	case FormatBAD:
		return mkError(ErrMidi, "Invalid format")
	case FormatCMF:
		if ErrBuiltin != nil {
			return ErrBuiltin
		}
	}

	type mkinfo func(io.ReadSeeker, []byte) (midinfo, error)

	var meta midinfo

	meta, err = [...]mkinfo{
		FormatGMF: s.metaGMF,
		FormatMUS: s.metaMUS,
		FormatCMF: s.metaCMF,
		FormatRSX: s.metaRSX,
		FormatIMF: s.metaIMF,
		FormatSMF: s.metaSMF,
	}[format](fd, hdr)

	if err != nil {
		return err
	}

	err = s.load(fd, format, meta)

	if err != nil {
		return err
	}

	s.loop.start = true
	return s.synth.prepareFilePlayback()
}

type Format int

const (
	FormatGMF Format = iota // General MIDI Format
	FormatCMF               // Creative Music Format
	FormatIMF               // id Software IMF
	FormatRSX               //  RSXX
	FormatXMD               // Extended MIDI
	FormatKLM               // KLM format
	FormatHMI               // HMI format
	FormatMUS               // MUS format

	FormatSMF // Standard MIDI File

	FormatBAD
)

func (f Format) String() string {
	strings := []string{
		"BAD",
		"GMF",
		"MUS",
		"CMF",
		"RSX",
		"IMF",
		"SMF",
	}

	if int(f) >= len(strings) {
		return "UNKNOWN"
	}

	return strings[f]
}

func (s *Sequencer) load(in io.ReadSeeker, format Format, info midinfo) error {
	δticks, ntrack := info.δticks, info.ntrack
	if δticks == 0 || ntrack == 0 {
		return mkError(ErrMidi, "invalid track count or time division")
	}
	s.synth.setFileFormat(format)

	// fmt.Fprintf(os.Stderr, "load: δticks=%d ntrack=%d eof=%d\n", δticks, ntrack, info.eof)

	s.tracks = make([][]byte, ntrack)
	s.pos.cur.tracks = make([]trackInfo, ntrack)
	s.invδ = newFrac(1, 1000000*int64(δticks))
	s.tempo = newFrac(1, int64(δticks))

	// fmt.Fprintf(os.Stderr, "load: tracks=%d\n", len(m.tracks))

	endTag := []byte{0xFF, 0x2F, 0x00, 0x00}

	for i := range ntrack {
		var (
			data []byte
			err  error
		)

		s.music = format

		switch format {
		case FormatIMF:
			data, err = s.readIMFTrack(in, info.eof)
		case FormatGMF, FormatCMF, FormatRSX:
			data, err = readRemaining(in)
		case FormatMUS:
			data, err = readMUSTrack(in)
		case FormatSMF:
			data, err = readSMFTrack(in, i)
		default:
			return mkError(ErrMidi, fmt.Sprintf("load: unknown format %d", format))
		}

		if err != nil {
			return mkError(ErrMidi, fmt.Sprintf("load: track %d: %v", i, err))
		}

		// Append the end-of-track meta event *before* building the reader so
		// the reader always sees a stable backing array.
		if format == FormatGMF || format == FormatMUS {
			data = append(data, endTag...)
		}

		s.tracks[i] = data
		s.pos.cur.tracks[i].Reader = bytes.NewReader(data)

		// fmt.Fprintf(os.Stderr, "load: track %d length=%d\n", i, len(data))

		switch format {
		case FormatIMF:
			s.pos.cur.tracks[i].delay = 0
			s.pos.cur.isPlaying = true
		case FormatRSX:
			// RSX tracks don't carry an initial delta.
		default:
			s.pos.cur.tracks[i].delay = s.ReadDelay(i)
		}
	}

	return nil
}

func (s *Sequencer) readIMFTrack(in io.ReadSeeker, eof int64) ([]byte, error) {
	const (
		imfTempo = uint32(1428)
	)

	// Header: a single tempo meta event followed by a delta.
	var out []byte
	out = append(out, 0xFF, 0x51, 0x03,
		byte(imfTempo>>16), byte(imfTempo>>8), byte(imfTempo&0xFF))
	out = append(out, 0x00)

	if _, err := in.Seek(2, io.SeekStart); err != nil {
		return nil, err
	}

	for {
		off, err := in.Seek(0, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		if off >= eof {
			break
		}

		var b [4]byte
		if _, err := io.ReadFull(in, b[:]); err != nil {
			// Tolerate a partial final event.
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, err
		}

		// 0xFF 0xE3 0x02 <lo> <hi> — "MIDI/IMF event" carrier.
		out = append(out, 0xFF, 0xE3, 0x02, b[0], b[1])

		delay := uint32(b[2]) + 256*uint32(b[3])
		if delay>>14 != 0 {
			out = append(out, byte(0x80|((delay>>14)&0x7F)))
		}

		if delay>>7 != 0 {
			out = append(out, byte(0x80|((delay>>7)&0x7F)))
		}

		out = append(out, byte(delay&0x7F))
	}

	out = append(out, 0xFF, 0x2F, 0x00, 0x00)
	return out, nil
}

func readRemaining(in io.ReadSeeker) ([]byte, error) {
	off0, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	end, err := in.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}

	if end < off0 {
		err = fmt.Errorf("negative track length: end=%d off0=%d", end, off0)
		return nil, err
	}

	if _, err := in.Seek(off0, io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, end-off0)
	if _, err := io.ReadFull(in, data); err != nil {
		return nil, err
	}
	return data, nil
}

func readMUSTrack(in io.ReadSeeker) ([]byte, error) {
	off0, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}

	if _, err := in.Seek(4, io.SeekStart); err != nil {
		return nil, err
	}

	var buf [2]byte
	if _, err := io.ReadFull(in, buf[:]); err != nil {
		return nil, err
	}
	tracklen := int(u16(buf[:]))

	if _, err := in.Seek(off0, io.SeekStart); err != nil {
		return nil, err
	}

	data := make([]byte, tracklen)
	if _, err := io.ReadFull(in, data); err != nil {
		return nil, err
	}
	return data, nil
}

func readSMFTrack(in io.ReadSeeker, i int) ([]byte, error) {
	var hdr [8]byte

	if _, err := io.ReadFull(in, hdr[:]); err != nil {
		err := fmt.Errorf("reading track header: %w", err)
		return nil, err
	}

	if string(hdr[:4]) != "MTrk" {
		err := fmt.Errorf("invalid track header at track %d: % x (%q)", i, hdr[:4], hdr[:4])
		return nil, mkError(ErrMidi, err)
	}

	tracklen := int(U32(hdr[4:]))

	data := make([]byte, tracklen)
	if _, err := io.ReadFull(in, data); err != nil {
		err := fmt.Errorf("reading track body (%d bytes): %w", tracklen, err)
		return nil, err
	}

	return data, nil
}

type midinfo struct {
	δticks uint32
	ntrack int
	eof    int64
}

func (s *Sequencer) metaGMF(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	return midinfo{δticks: 192, ntrack: 1}, nil
}

func (s *Sequencer) metaMUS(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	// Implement MUS loading logic here
	return midinfo{δticks: 192, ntrack: 1}, nil
}

func (s *Sequencer) metaCMF(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	ioff := int64(u16(hdr[6:]))
	moff := int64(u16(hdr[8:]))

	ntick := u16(hdr[12:])

	fp.Seek(22, io.SeekCurrent) // Skip channels-in-use table

	var raw [4]byte

	if _, err := io.ReadFull(fp, raw[:]); err != nil {
		return midinfo{}, err
	}

	nins := u16(raw[:2])

	fp.Seek(ioff, io.SeekStart)
	for range nins {
		var dat [16]byte

		if _, err := io.ReadFull(fp, dat[:]); err != nil {
			return midinfo{}, err
		}

		modE862 := uint32(dat[8])<<24 + uint32(dat[6])<<16 + uint32(dat[4])<<8 + uint32(dat[0])
		carE862 := uint32(dat[9])<<24 + uint32(dat[7])<<16 + uint32(dat[5])<<8 + uint32(dat[1])

		adl := adlData{
			ModulatorE862: modE862,
			CarrierE862:   carE862,
			Modulator40:   dat[2],
			Carrier40:     dat[3],
			FeedConn:      dat[10],
			Finetune:      0,
		}

		s.synth.addCMFInstrument(adl)
	}

	fp.Seek(moff, io.SeekStart)

	s.synth.useCMFBank()

	return midinfo{δticks: uint32(ntick), ntrack: 1}, nil
}

func (s *Sequencer) metaRSX(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	s.synth.useRSXXDefaults()

	return midinfo{δticks: 60, ntrack: 1}, nil
}

func (s *Sequencer) metaIMF(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	end := int64(u16(hdr[:2]))

	if end != 0 && (end&3) == 0 {
		var sum1, sum2 uint32

		off0, _ := fp.Seek(0, io.SeekCurrent)

		fp.Seek(2, io.SeekStart)

		b1 := make([]byte, 2)
		b2 := make([]byte, 2)

		for range 42 {
			if _, err := io.ReadFull(fp, b1); err != nil {
				return midinfo{}, err
			}

			v1 := uint32(b1[0]) + uint32(b1[1])<<8
			sum1 += v1

			if _, err := io.ReadFull(fp, b2); err != nil {
				return midinfo{}, err
			}

			v2 := uint32(b2[0]) + uint32(b2[1])<<8
			sum2 += v2
		}

		fp.Seek(off0, io.SeekStart)
	}

	end = int64(hdr[0]) + 256*int64(hdr[1])

	return midinfo{δticks: 1, ntrack: 1, eof: end}, nil
}

func (s *Sequencer) metaSMF(fp io.ReadSeeker, hdr []byte) (midinfo, error) {
	ntrack := int(U16(hdr[10:]))
	δticks := uint32(U16(hdr[12:]))

	return midinfo{δticks, ntrack, 0}, nil
}

func (s *Sequencer) detectFormat(fp io.ReadSeeker, hdr []byte) Format {
	hdrsz := int64(len(hdr))

	switch string(hdr[:4]) {
	case "GMF\x01":
		fp.Seek(7-hdrsz, io.SeekCurrent)

		return FormatGMF
	case "MUS\x1A":
		start := int64(u16(hdr[8:]))
		fp.Seek(start-hdrsz, 1)

		return FormatMUS
	case "CTMF":
		fp.Seek(22, io.SeekCurrent) // Skip unused bytes

		return FormatCMF
	default:
		if string(hdr[:4]) == "MThd" {
			return FormatSMF
		}

		var rsxx [8]byte

		off0, _ := fp.Seek(0, io.SeekCurrent)

		fp.Seek(int64(hdr[0])-0x10, io.SeekStart)

		if _, err := io.ReadFull(fp, rsxx[:]); err != nil {
			fp.Seek(off0, io.SeekStart)
			return FormatBAD
		}

		switch string(rsxx[:4]) {
		case "rsxx":
			fp.Seek(int64(hdr[0]), io.SeekStart)
			return FormatRSX
		default:
			// Not RSX: the probe moved the cursor, put it back after the header.
			fp.Seek(off0, io.SeekStart)
		}

		// Check for IMF format as a fallback
		end := uint32(u16(hdr[:2]))

		if end != 0 && (end&3) == 0 {
			var (
				sum0, sum1 uint32
				buf        [2]byte
			)

			off0, _ = fp.Seek(0, io.SeekCurrent)
			defer fp.Seek(off0, io.SeekStart)

			fp.Seek(2, io.SeekStart)

			for range 42 {
				if _, err := io.ReadFull(fp, buf[:]); err != nil {
					return FormatBAD
				}

				v0 := uint32(buf[0]) + uint32(buf[1])<<8
				sum0 += v0

				if _, err := io.ReadFull(fp, buf[:]); err != nil {
					return FormatBAD
				}

				v1 := uint32(buf[0]) + uint32(buf[1])<<8
				sum1 += v1
			}

			if sum0 > sum1 {
				return FormatIMF
			}
		}

		if string(hdr[:4]) == "MThd" {
			return FormatSMF
		}

		return FormatBAD
	}
}

// Tick advances playback by dt and returns the time until the next event.
func (s *Sequencer) Tick(dt time.Duration) time.Duration {
	wasPlaying := s.pos.cur.isPlaying
	if wasPlaying {
		s.pos.cur.wait -= dt
	}

	// Process events until the next one is in the future. Guard against a
	// zero-progress loop in case ProcessEvents can't advance, and stop as
	// soon as playback is asked to quit.
	// Before the first note-on Wait is not credited, so keep going (bounded).
	for n := 0; s.pos.cur.wait <= 0 && n < 1<<20 && !s.quit.flag; n++ {
		before := s.pos.cur.wait
		loop := s.loop.generation

		s.ProcessEvents()

		if s.pos.cur.isPlaying && s.pos.cur.wait == before && s.loop.generation == loop {
			break
		}
	}

	return s.pos.cur.wait
}

func readLen(fp *bytes.Reader) (v uint32, err error) {
	var x byte

	for range 4 {
		x, err = fp.ReadByte()

		if err != nil {
			return
		}

		v = (v << 7) | uint32(x&0x7F)

		if x&0x80 == 0 {
			break
		}
	}

	return
}

// ProcessEvents processes MIDI events
func (s *Sequencer) ProcessEvents() {
	s.loop.end = false

	ntrack := len(s.tracks)

	row0 := s.pos.cur.clone()
	tempo0 := s.tempo

	tracks := s.pos.cur.tracks

	// Pass 1: drain all ready events on every track, and find the
	// shortest positive delay remaining.
	var shortest int64 = math.MaxInt64

trackScan:
	for i := range ntrack {
		tr := &tracks[i]

		if tr.status < 0 {
			continue
		}

		for tr.delay <= 0 {
			s.HandleEvent(uint32(i))

			if tr.status < 0 {
				continue trackScan
			}

			tr.delay += s.ReadDelay(i)
		}

		shortest = min(shortest, tr.delay)
	}

	// Pass 2: advance every active track's delay toward zero.
	if shortest < math.MaxInt64 {
		for tk := range ntrack {
			tr := &tracks[tk]

			if tr.status < 0 {
				continue
			}

			tr.delay -= shortest
		}
	}

	if s.pos.cur.isPlaying && shortest < math.MaxInt64 && !s.loop.end {
		wait := frac{num: shortest * s.tempo.num, den: s.tempo.den}

		s.pos.cur.wait += time.Duration(wait.eval() * float64(time.Second))
	}

	if s.loop.start {
		s.pos.loop = row0
		s.loop.tempo = tempo0
		s.loop.start = false
	}

	if shortest == math.MaxInt64 || s.loop.end {
		s.loop.end = false

		if s.quit.noloop {
			s.quit.flag = true
			return
		}

		s.pos.cur = s.pos.loop.clone()
		s.pos.cur.wait = row0.wait
		s.tempo = s.loop.tempo
		s.loop.generation++
	}
}

// HandleEvent handles a single MIDI event
func (s *Sequencer) HandleEvent(trackID uint32) {
	tr := &s.pos.cur.tracks[trackID]

	tracklen := int64(len(s.tracks[trackID]))

	x, _ := tr.ReadByte()

	if x == 0xF7 || x == 0xF0 { // SysEx event
		length, err := readLen(tr.Reader)

		if err != nil {
			tr.status = -1

			return
		}

		off, _ := tr.Seek(0, io.SeekCurrent)

		if off+int64(length) > tracklen {
			// Truncated / corrupt event — bail out safely.
			tr.status = -1
			tr.Seek(tracklen, io.SeekStart)

			return
		}

		if x == 0xF0 {
			s.synth.SysEx(s.curDev[trackID], s.tracks[trackID][off:off+int64(length)])
		}

		_, _ = tr.Seek(int64(length), io.SeekCurrent)

		return
	}

	if x == 0xFF { // Meta event
		metatype, _ := tr.ReadByte()

		metasz, _ := readLen(tr.Reader)
		off, _ := tr.Seek(0, io.SeekCurrent)

		if off+int64(metasz) > tracklen {
			// Truncated / corrupt event — bail out safely.
			tr.status = -1
			tr.Seek(tracklen, io.SeekStart)

			return
		}

		meta := s.tracks[trackID][off : off+int64(metasz)]
		tr.Seek(int64(metasz), io.SeekCurrent)

		switch metatype {
		case 0x2F: // End of track
			tr.status = -1
		case 0x51: // Set tempo
			x := int64(readBEInt(meta, len(meta)))
			s.tempo = s.invδ.mul(newFrac(x, 1))
		case 0x06: // Marker / text event
			switch string(meta) {
			case "loopStart":
				s.loop.start = true
			case "loopEnd":
				s.loop.end = true
			}
		case 0x09: // Device selection
			s.curDev[trackID] = s.synth.DeviceSelect(string(meta))
		case 0xE3: // OPL-specific event
			i, v := int(meta[0]), int(meta[1])

			if (i & 0xF0) == 0xC0 {
				v |= 0x30
			}

			s.synth.writeOPL(i, v)
		}

		return
	}

	// Running status
	if x < 0x80 {
		x = uint8(tr.status | 0x80)
		tr.UnreadByte()
	}

	switch x {
	case 0xF3: // Song select
		_, _ = tr.ReadByte() // discard song select value
		return
	case 0xF2: // Song position pointer
		_, _ = tr.ReadByte() // discard LSB of song position pointer
		_, _ = tr.ReadByte() // discard MSB of song position pointer
		return
	}

	ii := uint32(x&0x0F) + s.curDev[trackID]

	tr.status = int(x)

	s.handleMIDIEvent(tr, x, ii)
}
