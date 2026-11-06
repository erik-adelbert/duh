package mid

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"time"
)

// const (
// 	FlagQuadOp = 1 << iota
// 	FlagNoSound
// )

// MIDIDecoder represents the main MIDI player
type MIDIDecoder struct {
	// Information about each track

	pos struct {
		cur, loop Position
	}

	Devices map[string]uint32
	CurDev  map[uint32]uint32 // track -> channel begin index

	// Persistent settings for each MIDI channel
	chans []Channel
	// chmod ChanMode

	CMFDrum bool

	// Additional information about AdLib channels
	ADLChans []ADLChannel

	tracks [][]byte

	invδ  Fraction
	tempo Fraction

	loop struct {
		start, end bool
	}

	OPL *OPL3

	// Settings
	adlBank uint32
	vol     struct {
		log  bool
		toon bool
	}

	music Format

	mode  SynthMode
	devID uint8 // SysEx device id

	ADLDrum bool

	percussionMap [256]int

	quit struct {
		noloop bool
		flag   bool
	}

	dyn struct {
		ins   []ADLData
		metas []ADLMeta
	}

	arpeggio uint32
}

const NOID = ^uint16(0)

// Position tracks the playback position
type Position struct {
	ID uint16

	Playing bool

	Wait time.Duration

	// TickWait     uint64
	// Tick0, TickN uint64

	Tracks []TrackInfo
}

// TrackInfo holds info about a single track's position
type TrackInfo struct {
	*bytes.Reader
	Delay  int64
	Status int

	// LastEvent int
	// State0    TrackDump
}

// ADLChannel represents an AdLib channel
type ADLChannel struct {
	Users     map[Location]LocationData
	KOFFDecay time.Duration
}

// Location identifies a note on a MIDI channel
type Location struct {
	MidCh uint16
	Note  uint8
}

// Hold tells which pedals keep a released note sounding.
type Hold uint8

const (
	HoldNone      Hold = 0
	HoldPedal     Hold = 1 // sustain pedal
	HoldSostenuto Hold = 2 // sostenuto pedal
	HoldAny       Hold = HoldPedal | HoldSostenuto
)

// LocationData holds data for a location
type LocationData struct {
	Sustained Hold
	Ins       uint16
	KONDecay  time.Duration
	VibDelay  time.Duration
}

// NewMIDIDecoder creates a new MIDI player
func NewMIDIDecoder(cardCount, quadOpsCount, flags int) *MIDIDecoder {
	// fmt.Fprintln(os.Stderr, "creating new MIDI decoder")

	m := &MIDIDecoder{
		Devices: make(map[string]uint32),
		CurDev:  make(map[uint32]uint32),
		OPL:     NewOPL3(cardCount, quadOpsCount, flags),
		adlBank: uint32(BankSelect),
		mode:    ModeXG,
	}

	m.tempo = NewFraction(1, 192)
	m.invδ = NewFraction(1, 1000000*192)

	m.SetVolumeModel(0)

	return m
}

// SetVolumeModel selects the volume model: 0 picks the one declared by the
// current bank, 1..14 select a model like libADLMIDI's -vm option
// (1 generic, 2 native, 3 DMX, ... 14 O'Connell).
func (m *MIDIDecoder) SetVolumeModel(id int) {
	if id == 0 {
		id = BankVolumeModel(int(m.adlBank)) + 1
	}

	m.OPL.SetVolumeModel(NewVolumeModel(id - 1))
}

// defaults returns the default channel volume and pitch-bend sensitivity:
// MT-32 banks and RSXX files start at full volume with a 12 semitones range,
// like libADLMIDI's resetMIDIDefaults.
func (m *MIDIDecoder) defaults() (vol, bend uint8) {
	if m.vol.toon || m.mt32() {
		return 127, 12
	}

	return 100, 2
}

// mt32 reports whether the current bank declares MT-32 defaults.
func (m *MIDIDecoder) mt32() bool {
	bank := int(m.adlBank)

	return bank >= 0 && bank < len(Banks) && Banks[bank].Setup&(1<<10) != 0
}

// SetDefaults applies the default volume and bend sensitivity to every channel.
func (m *MIDIDecoder) SetDefaults() {
	vol, bend := m.defaults()

	for ii := range m.chans {
		ch := &m.chans[ii]

		ch.DefVolume, ch.DefBend = vol, bend
		ch.Volume = vol
		ch.Pitch.MSB = int(bend)
	}
}

// SetNoLooping controls whether playback stops when the song reaches its end.
func (m *MIDIDecoder) SetNoLooping(noLoop bool) {
	m.quit.noloop = noLoop
}

func (m *MIDIDecoder) Quit() {
	m.quit.flag = true
}

// IsFinished reports whether playback has reached the end in no-loop mode.
func (m *MIDIDecoder) IsFinished() bool {
	return m.quit.flag
}

// ReadDelay reads a delta time. Like libADLMIDI's end-silence skipping, a
// delay that only precedes the end-of-track event is dropped: some files pad
// a track with minutes of silence that would otherwise be played.
func (m *MIDIDecoder) ReadDelay(tk int) int64 {
	delay := int64(m.ReadVar(tk))

	if delay > 0 {
		tr := m.pos.cur.Tracks[tk]

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
func (m *MIDIDecoder) ReadVar(tk int) (v uint32) {
	tr := m.pos.cur.Tracks[tk].Reader

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

func (m *MIDIDecoder) Open(filename string) (err error) {
	defer func() {
		err = mkError(ErrMidi, err)
	}()

	fd, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

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
	format := m.detectFormat(fd, hdr)

	switch format {
	case FormatBAD:
		return mkError(ErrMidi, "Invalid format")
	case FormatCMF:
		if builtinErr != nil {
			return builtinErr
		}
	}

	type mkinfo func(*os.File, []byte) (midinfo, error)

	var meta midinfo

	meta, err = [...]mkinfo{
		FormatGMF: m.metaGMF,
		FormatMUS: m.metaMUS,
		FormatCMF: m.metaCMF,
		FormatRSX: m.metaRSX,
		FormatIMF: m.metaIMF,
		FormatSMF: m.metaSMF,
	}[format](fd, hdr)

	if err != nil {
		return err
	}

	err = m.load(fd, format, meta)

	if err != nil {
		return err
	}

	return m.initADLChannels()
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

func (m *MIDIDecoder) load(in *os.File, format Format, info midinfo) error {
	δticks, ntrack := info.δticks, info.ntrack

	// fmt.Fprintf(os.Stderr, "load: δticks=%d ntrack=%d eof=%d\n", δticks, ntrack, info.eof)

	m.tracks = make([][]byte, ntrack)
	m.pos.cur.Tracks = make([]TrackInfo, ntrack)
	m.invδ = NewFraction(1, 1000000*int64(δticks))
	m.tempo = NewFraction(1, int64(δticks))

	// fmt.Fprintf(os.Stderr, "load: tracks=%d\n", len(m.tracks))

	endTag := []byte{0xFF, 0x2F, 0x00, 0x00}

	for i := range ntrack {
		var (
			data []byte
			err  error
		)

		m.music = format

		switch format {
		case FormatIMF:
			data, err = m.readIMFTrack(in, info.eof)
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

		m.tracks[i] = data
		m.pos.cur.Tracks[i].Reader = bytes.NewReader(data)

		// fmt.Fprintf(os.Stderr, "load: track %d length=%d\n", i, len(data))

		switch format {
		case FormatIMF:
			m.pos.cur.Tracks[i].Delay = 0
			m.pos.cur.Playing = true
		case FormatRSX:
			// RSX tracks don't carry an initial delta.
		default:
			m.pos.cur.Tracks[i].Delay = m.ReadDelay(i)
		}
	}

	return nil
}

func (m *MIDIDecoder) readIMFTrack(in *os.File, eof int64) ([]byte, error) {
	const (
		imfTempo = uint32(1428)
	)

	// Header: a single tempo meta event followed by a delta.
	var out []byte
	out = append(out, 0xFF, 0x51, 0x04,
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

func readRemaining(in *os.File) ([]byte, error) {
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

func readMUSTrack(in *os.File) ([]byte, error) {
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

func readSMFTrack(in *os.File, i int) ([]byte, error) {
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

func (m *MIDIDecoder) initADLChannels() error {
	m.loop.start = true

	m.ADLChans = make([]ADLChannel, m.OPL.ChanCount())
	for i := range m.ADLChans {
		m.ADLChans[i].Users = make(map[Location]LocationData)
	}

	return nil
}

type midinfo struct {
	δticks uint32
	ntrack int
	eof    int64
}

func (m *MIDIDecoder) metaGMF(fp *os.File, hdr []byte) (midinfo, error) {
	return midinfo{δticks: 192, ntrack: 1}, nil
}

func (m *MIDIDecoder) metaMUS(fp *os.File, hdr []byte) (midinfo, error) {
	// Implement MUS loading logic here
	return midinfo{δticks: 192, ntrack: 1}, nil
}

func (m *MIDIDecoder) metaCMF(fp *os.File, hdr []byte) (midinfo, error) {
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

		adl := ADLData{
			ModulatorE862: modE862,
			CarrierE862:   carE862,
			Modulator40:   dat[2],
			Carrier40:     dat[3],
			FeedConn:      dat[10],
			Finetune:      0,
		}

		adlins := ADLMeta{
			ADLID0: uint16(len(m.dyn.ins)) | 0x8000,
			ADLID1: uint16(len(m.dyn.ins)) | 0x8000,
			KON:    1000,
			KOFF:   500,
			Tone:   0,
			Flags:  0,
		}

		m.dyn.metas = append(m.dyn.metas, adlins)
		m.dyn.ins = append(m.dyn.ins, adl)
	}

	fp.Seek(moff, io.SeekStart)

	m.adlBank = ^uint32(0) // Use dynamic banks

	m.vol.log = true

	return midinfo{δticks: uint32(ntick), ntrack: 1}, nil
}

func (m *MIDIDecoder) metaRSX(fp *os.File, hdr []byte) (midinfo, error) {
	m.vol.log = true
	m.vol.toon = true
	m.SetDefaults()

	return midinfo{δticks: 60, ntrack: 1}, nil
}

func (m *MIDIDecoder) metaIMF(fp *os.File, hdr []byte) (midinfo, error) {
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

func (m *MIDIDecoder) metaSMF(fp *os.File, hdr []byte) (midinfo, error) {
	ntrack := int(U16(hdr[10:]))
	δticks := uint32(U16(hdr[12:]))

	return midinfo{δticks, ntrack, 0}, nil
}

func (m *MIDIDecoder) detectFormat(fp *os.File, hdr []byte) Format {
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
func (m *MIDIDecoder) Tick(dt time.Duration) time.Duration {
	wasPlaying := m.pos.cur.Playing
	if wasPlaying {
		m.pos.cur.Wait -= dt
	}

	// Process events until the next one is in the future. Guard against a
	// zero-progress loop in case ProcessEvents can't advance, and stop as
	// soon as playback is asked to quit.
	// Before the first note-on Wait is not credited, so keep going (bounded).
	for n := 0; m.pos.cur.Wait <= 0 && n < 1<<20 && !m.quit.flag; n++ {
		before := m.pos.cur.Wait

		m.ProcessEvents()

		if m.pos.cur.Playing && m.pos.cur.Wait == before {
			break
		}
	}

	// Channel timers only run while the song is playing.
	if wasPlaying {
		for c := range m.ADLChans {
			m.ADLChans[c].Advance(dt)
		}

		m.UpdateGlide(dt)
		m.UpdateVibrato(dt)
		m.UpdateArpeggio(dt)
	}

	return m.pos.cur.Wait
}

const minDecay = -0x1FFFFFFF * time.Millisecond

// Advance advances the channel's per-user decay/vibrato timers by dt.
func (ac *ADLChannel) Advance(dt time.Duration) {
	if len(ac.Users) == 0 {
		ac.KOFFDecay = max(ac.KOFFDecay-dt, minDecay)
		return
	}

	ac.KOFFDecay = 0
	for k, v := range ac.Users {
		v.KONDecay = max(v.KONDecay-dt, minDecay)
		v.VibDelay += dt
		ac.Users[k] = v
	}
}

// Update flags
const (
	UpdPatch  = 0x1
	UpdPan    = 0x2
	UpdVolume = 0x4
	UpdPitch  = 0x8
	UpdAll    = UpdPan + UpdVolume + UpdPitch
	UpdOff    = 0x20
)

// NoteUpdate updates a note
func (m *MIDIDecoder) NoteUpdate(midCh uint32, note uint8, propsMask uint32, ADLChan int) {
	info, ok := m.chans[midCh].ActiveNotes[note]

	if !ok {
		return
	}

	vol := info.Vol

	meta := info.Instrument.Meta
	ains := m.GetAdlMeta(meta)
	if has(propsMask, UpdVolume) {
		vol = uint8(max(0, min(127, int(vol)+int(ains.VelocityOffset))))
	}

	loc := Location{MidCh: uint16(midCh), Note: note}

	for ch, ins := range info.Phys {
		if ADLChan >= 0 && ch != ADLChan {
			continue
		}

		if has(propsMask, UpdPatch) {
			m.OPL.Patch(ch, ins)

			d := m.ADLChans[ch].Users[loc]

			d.Sustained = HoldNone
			d.VibDelay = 0
			d.KONDecay = ains.KON
			d.Ins = ins

			m.ADLChans[ch].Users[loc] = d
		}
	}

	for ch, ins := range info.Phys {
		if ADLChan >= 0 && ch != ADLChan {
			continue
		}

		if has(propsMask, UpdOff) {
			if m.chans[midCh].Sustain == 0 {
				// A sostenuto-held note stays until the pedal is released
				if m.ADLChans[ch].Users[loc].Sustained&HoldSostenuto == 0 {
					delete(m.ADLChans[ch].Users, loc)

					if len(m.ADLChans[ch].Users) == 0 {
						m.OPL.NoteOff(ch)

						m.ADLChans[ch].KOFFDecay = ains.KOFF
					}
				}
			} else {
				d := m.ADLChans[ch].Users[loc]
				d.Sustained |= HoldPedal

				m.ADLChans[ch].Users[loc] = d
			}

			delete(info.Phys, ch)
			continue
		}

		if has(propsMask, UpdPan) {
			m.OPL.Pan(ch, int(m.chans[midCh].Panning))
		}

		if has(propsMask, UpdVolume) {
			c := &m.chans[midCh]

			// Brightness acts like a post high-pass filter: it only
			// attenuates by half its range, like libADLMIDI's default.
			bright := int(c.Brightness)
			if bright >= 64 {
				bright = 127
			} else {
				bright *= 2
			}

			m.OPL.Touch(ch, int(vol), int(c.Volume), int(c.Expression), bright, c.IsDrum(midCh))
		}

		if has(propsMask, UpdPitch) {
			d := m.ADLChans[ch].Users[loc]

			if d.Sustained == HoldNone {
				var phase float64

				pitch := m.chans[midCh].Pitch.Value() + float64(m.GetAdlIns(ins).Finetune)

				if Is4Op(ains.Flags) && ins == ains.ADLID1 {
					phase = 0.125
					pitch += float64(ains.SecondFinetune)
				}

				vib := m.chans[midCh].Vibrato
				level := max(vib.Level, m.chans[midCh].Aftertouch, info.Pressure)

				if level != 0 && d.VibDelay >= vib.Delay {
					pitch += float64(level) * vib.Depth *
						math.Sin(vib.Pos)
				}

				m.OPL.NoteOn(ch, info.Glide+pitch+phase)
			}
		}
	}

	if len(info.Phys) == 0 {
		delete(m.chans[midCh].ActiveNotes, note)
	}
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
func (m *MIDIDecoder) ProcessEvents() {
	m.loop.end = false

	ntrack := len(m.tracks)

	row0 := m.pos.cur

	tracks := m.pos.cur.Tracks

	// Pass 1: drain all ready events on every track, and find the
	// shortest positive delay remaining.
	var shortest int64 = math.MaxInt64

trackScan:
	for i := range ntrack {
		tr := &tracks[i]

		if tr.Status < 0 {
			continue
		}

		for tr.Delay <= 0 {
			m.HandleEvent(uint32(i))

			if tr.Status < 0 {
				continue trackScan
			}

			tr.Delay += m.ReadDelay(i)
		}

		shortest = min(shortest, tr.Delay)
	}

	// Pass 2: advance every active track's delay toward zero.
	if shortest < math.MaxInt64 {
		for tk := range ntrack {
			tr := &tracks[tk]

			if tr.Status < 0 {
				continue
			}

			tr.Delay -= shortest
		}
	}

	if m.pos.cur.Playing {
		wait := Fraction{Num: shortest * m.tempo.Num, Den: m.tempo.Den}

		m.pos.cur.Wait += time.Duration(wait.Eval() * float64(time.Second))
	}

	if m.loop.start {
		m.pos.loop = row0
		m.loop.start = false
	}

	if shortest == math.MaxInt64 || m.loop.end {
		m.loop.end = false

		if m.quit.noloop {
			m.quit.flag = true
			return
		}

		m.pos.cur = m.pos.loop
	}
}

// HandleEvent handles a single MIDI event
func (m *MIDIDecoder) HandleEvent(trackID uint32) {
	tr := &m.pos.cur.Tracks[trackID]

	tracklen := int64(len(m.tracks[trackID]))

	x, _ := tr.ReadByte()

	if x == 0xF7 || x == 0xF0 { // SysEx event
		length, err := readLen(tr.Reader)

		if err != nil {
			tr.Status = -1

			return
		}

		off, _ := tr.Seek(0, io.SeekCurrent)

		if off+int64(length) > tracklen {
			// Truncated / corrupt event — bail out safely.
			tr.Status = -1
			tr.Seek(tracklen, io.SeekStart)

			return
		}

		if x == 0xF0 {
			m.SysEx(m.CurDev[trackID], m.tracks[trackID][off:off+int64(length)])
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
			tr.Status = -1
			tr.Seek(tracklen, io.SeekStart)

			return
		}

		meta := m.tracks[trackID][off : off+int64(metasz)]
		tr.Seek(int64(metasz), io.SeekCurrent)

		switch metatype {
		case 0x2F: // End of track
			tr.Status = -1
		case 0x51: // Set tempo
			x := int64(ReadBEInt(meta, len(meta)))
			m.tempo = m.invδ.Mul(NewFraction(x, 1))
		case 0x06: // Marker / text event
			switch string(meta) {
			case "loopStart":
				m.loop.start = true
			case "loopEnd":
				m.loop.end = true
			}
		case 0x09: // Device selection
			m.CurDev[trackID] = m.ChooseDevice(string(meta))
		case 0xE3: // OPL-specific event
			i, v := int(meta[0]), int(meta[1])

			if (i & 0xF0) == 0xC0 {
				v |= 0x30
			}

			m.OPL.Poke(0, i, v)
		}

		return
	}

	// Running status
	if x < 0x80 {
		x = uint8(tr.Status | 0x80)
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

	ii := uint32(x&0x0F) + m.CurDev[trackID]
	ch := &m.chans[ii]

	evtype := x >> 4

	tr.Status = int(x)

	switch evtype {
	case 0x08, 0x09: // Note off/on
		note, _ := tr.ReadByte()
		vol, _ := tr.ReadByte()

		if m.vol.toon && vol != 0 {
			if cur, ok := ch.ActiveNotes[note]; ok {
				cur.Vol = vol
				m.NoteUpdate(ii, note, UpdVolume, -1)
				break
			}
		}

		m.NoteOff(ii, note)

		if vol == 0 || evtype == 0x08 {
			break
		}

		patch := ch.Patch
		if ch.IsDrum(ii) {
			patch = 0x80 + note
		}

		id := m.GetAdlMetaID(patch, ch)
		meta := m.GetAdlMeta(id)

		tone := int16(note)
		if meta.Tone != 0 {
			switch {
			case meta.Tone < 20:
				tone += int16(meta.Tone)
			case meta.Tone < 128:
				tone = int16(meta.Tone)
			default:
				tone -= int16(meta.Tone - 128)
			}
		}

		ids := [2]uint16{meta.ADLID0, meta.ADLID1}

		if m.ADLDrum && m.percussionMap[patch] != 0 {
			ids[1] = ids[0]
		}

		// Allocate AdLib channel
		var ADLChans [2]int
		ADLChans[0], ADLChans[1] = -1, -1

		for ch := range ids {
			if ch == 1 {
				if ids[0] == ids[1] {
					break
				}

				if ADLChans[0] == -1 {
					break
				}
			}

			pseudo4op := IsPseudo4Op(meta.Flags)

			best := -1
			var score int64 = math.MinInt64 // int64(-0x7FFFFFFF)
			for i := range m.OPL.ChanCount() {
				if ch == 1 && i == ADLChans[0] {
					continue
				}

				if ids[0] == ids[1] || pseudo4op {
					var want int

					if m.ADLDrum {
						want = m.percussionMap[patch]

						if m.CMFDrum && ii >= 11 {
							want = int(3 + ii - 11)
						}
					}

					if m.OPL.QuadOps[i] != int8(want) {
						continue
					}
				} else {
					switch {
					case ch == 0 && m.OPL.QuadOps[i] != 1:
						continue
					case ch == 1 && i != ADLChans[0]+3:
						continue
					}
				}

				sc := m.ChannelGoodness(uint32(i), uint32(ids[ch]), ii)
				if sc > score {
					score, best = sc, i
				}
			}

			if best < 0 {
				continue
			}

			m.PrepareForNewNote(best, int(ids[ch]))
			ADLChans[ch] = best
		}

		if ADLChans[0] < 0 && ADLChans[1] < 0 {
			break
		}

		// Allocate active note
		if ch.ActiveNotes == nil {
			ch.ActiveNotes = make(map[uint8]*NoteInfo, 8)
		}

		if ch.Soft { // soft pedal reduces the level
			vol = uint8(float32(vol) * 0.8)
		}

		info := &NoteInfo{
			Vol:  vol,
			Tone: tone,

			Pressure: ch.NoteAftertouch[note&0x7F],

			Glide:     float64(tone),
			GlideRate: math.Inf(1),

			Midi: patch,
			Meta: id,

			Phys: make(map[int]uint16),
		}

		// Glide from the previous note of the channel when portamento is on
		if ch.Enabled && !math.IsInf(ch.Rate, 1) && !ch.IsDrum(ii) && ch.Source >= 0 {
			info.Glide = float64(ch.Source)
			info.GlideRate = ch.Rate
		}

		ch.Source = int8(note)

		for ch := range ADLChans {
			c := ADLChans[ch]

			if c < 0 {
				continue
			}

			info.Phys[c] = ids[ch] // pack: [0]=ch0, [1]=ch1, [2]=ins0, [3]=ins1
		}

		ch.ActiveNotes[note] = info
		m.pos.cur.Playing = true
		m.NoteUpdate(ii, note, UpdAll|UpdPatch, -1)

	case 0x0A: // Note touch
		note, _ := tr.ReadByte()
		vol, _ := tr.ReadByte()

		if info, ok := ch.ActiveNotes[note]; ok {
			info.Pressure = vol
		}

		if old := ch.NoteAftertouch[note&0x7F]; vol != old {
			ch.NoteAftertouch[note&0x7F] = vol
			ch.NoteTouchInUse = vol != 0

			for _, v := range ch.NoteAftertouch {
				ch.NoteTouchInUse = ch.NoteTouchInUse || v != 0
			}
		}

	case 0x0B: // Controller change
		ctrl, _ := tr.ReadByte()
		val, _ := tr.ReadByte()

		switch ctrl {
		case 0: // Bank MSB
			ch.Bank.MSB = val
			ch.BankSelected = true

			if m.mode != ModeGS { // XG drums aren't used in GS mode
				ch.Percussion = ch.IsXGDrum()
			}
		case 1: // Modulation wheel / vibrato
			ch.Vibrato.Level = val

		case 5: // Portamento MSB
			ch.Portamento.Level = (ch.Portamento.Level & 0x7F) | (uint16(val) << 7)
			m.UpdatePortamento(ii)
		case 6: // Data entry MSB (RPN)
			m.SetRPN(ii, uint32(val), true)
		case 7: // Channel volume (MSB)
			ch.Volume = val
			m.NoteUpdateAll(ii, UpdVolume)
		case 10: // Pan
			ch.Panning = val
			m.NoteUpdateAll(ii, UpdPan)
		case 11: // Expression controller (MSB)
			ch.Expression = val
			m.NoteUpdateAll(ii, UpdVolume)
		case 32: // Bank LSB
			ch.Bank.LSB = val

			if m.mode != ModeGS {
				ch.Percussion = ch.IsXGDrum()
			}
			ch.BankSelected = true
		case 37: // Portamento LSB
			ch.Portamento.Level = (ch.Portamento.Level & 0x3F80) | uint16(val)
			m.UpdatePortamento(ii)
		case 38: // Data entry LSB (RPN)
			m.SetRPN(ii, uint32(val), false)
		case 64: // Sustain pedal
			ch.Sustain = val
			if val == 0 {
				m.KillSustainingNotes(int(ii), -1, HoldPedal)
			}
		case 66: // Sostenuto pedal
			if val >= 64 {
				m.MarkSostenutoNotes(int(ii))
			} else {
				m.KillSustainingNotes(int(ii), -1, HoldSostenuto)
			}
		case 67: // Soft pedal
			ch.Soft = val >= 64
		case 65: // Portamento enable/disable
			ch.Portamento.Enabled = val >= 64
			m.UpdatePortamento(ii)
		case 74: // Brightness
			ch.Brightness = val
			m.NoteUpdateAll(ii, UpdVolume)
		case 91, 92, 93, 94, 95: // Effect depths - not supported
			// Effect depths - not supported
		case 98: // NRPN LSB (Non-Registered Parameter Number)
			ch.LastLRPN = val
			ch.NRPN = true
		case 99: // NRPN MSB (Non-Registered Parameter Number)
			ch.LastMRPN = val
			ch.NRPN = true
		case 100: // RPN LSB (Registered Parameter Number)
			ch.LastLRPN = val
			ch.NRPN = false
		case 101: // RPN MSB (Registered Parameter Number)
			ch.LastMRPN = val
			ch.NRPN = false
		case 103: // Channel mode message (CMF Drum)
			m.CMFDrum = val != 0
		case 113: // Related to pitch-bender
			// Related to pitch-bender
		case 121: // Reset all controllers
			ch.ResetControllers()

			if m.vol.toon {
				ch.Volume = ch.DefVolume
			}

			m.UpdatePortamento(ii)
			m.NoteUpdateAll(ii, UpdPan|UpdVolume|UpdPitch)
			m.KillSustainingNotes(int(ii), -1, HoldAny)
		case 123: // All notes off
			m.NoteUpdateAll(ii, UpdOff)
		}

	case 0x0C: // Patch change
		ch.Patch, _ = tr.ReadByte()

	case 0x0D: // Channel after-touch
		ch.Aftertouch, _ = tr.ReadByte()

	case 0x0E: // Wheel/pitch bend
		a, _ := tr.ReadByte()
		b, _ := tr.ReadByte()

		ch.Pitch.Bend = float64(a) + float64(b)*128 - 8192
		m.NoteUpdateAll(ii, UpdPitch)
	}
}

// ChannelGoodness determines how good a candidate channel is
func (m *MIDIDecoder) ChannelGoodness(c0, ins, midCh uint32) int64 {
	s := -m.ADLChans[c0].KOFFDecay.Milliseconds()

	for loc, data := range m.ADLChans[c0].Users {
		s -= 4000

		if data.Sustained == HoldNone {
			s -= data.KONDecay.Milliseconds()
		} else {
			s -= data.KONDecay.Milliseconds() / 2
		}

		if info, ok := m.chans[loc.MidCh].ActiveNotes[loc.Note]; ok {
			if data.Ins == uint16(ins) {
				s += 300
			}

			s += 50 * int64(info.Instrument.Midi/128)

			var nevac int64

			for c1 := range m.OPL.ChanCount() {
				if c1 == int(c0) {
					continue
				}

				if m.OPL.QuadOps[c1] != m.OPL.QuadOps[c0] {
					continue
				}

				for _, mData := range m.ADLChans[c1].Users {
					if mData.Sustained != HoldNone || mData.VibDelay >= 200 || mData.Ins != data.Ins {
						continue
					}

					nevac++
				}
			}

			s += nevac * 4
		}
	}

	return s
}

// PrepareForNewNote prepares a channel for a new note
func (m *MIDIDecoder) PrepareForNewNote(c, ins int) {
	if len(m.ADLChans[c].Users) == 0 {
		return
	}

	for loc, data := range m.ADLChans[c].Users {
		if data.Sustained == HoldNone {
			if (data.VibDelay < 70 || data.KONDecay > 20000) && data.Ins == uint16(ins) {
				continue // Do arpeggio
			}
			m.MoveOrKill(uint32(c), loc, data)
		}
	}

	// Kill all sustained notes on this channel.
	// Don't keep them for arpeggio, because arpeggio requires
	// an intact "activenotes" record. This is a design flaw.
	m.KillSustainingNotes(-1, c, HoldAny)

	if len(m.ADLChans[c].Users) == 0 {
		m.OPL.NoteOff(c)
	}
}

// MoveOrKill kills or evacuates a note
func (m *MIDIDecoder) MoveOrKill(fromChannel uint32, loc Location, data LocationData) {
	for c := range m.OPL.ChanCount() {
		if uint32(c) == fromChannel {
			continue
		}

		if m.OPL.QuadOps[c] != m.OPL.QuadOps[fromChannel] {
			continue
		}

		for _, locdat := range m.ADLChans[c].Users {
			if locdat.VibDelay >= 200 && locdat.KONDecay < 10000 {
				continue
			}

			if locdat.Ins != data.Ins {
				continue
			}

			// Move the note to another channel if possible
			if info, ok := m.chans[loc.MidCh].ActiveNotes[loc.Note]; ok {
				delete(info.Phys, int(fromChannel))

				info.Phys[int(c)] = data.Ins

				m.ADLChans[c].Users[loc] = data
				delete(m.ADLChans[fromChannel].Users, loc)

				return
			}
		}
	}

	// Kill
	if _, ok := m.chans[loc.MidCh].ActiveNotes[loc.Note]; ok {
		m.NoteUpdate(uint32(loc.MidCh), loc.Note, UpdOff, int(fromChannel))
	}
}

// KillSustainingNotes releases the given kind of hold on notes, and kills
// those no longer held by any pedal.
func (m *MIDIDecoder) KillSustainingNotes(midCh, baseCh int, kind Hold) {
	first, last := 0, m.OPL.ChanCount()

	if baseCh >= 0 {
		first = baseCh
		last = first + 1
	}

	for ch := first; ch < last; ch++ {
		if len(m.ADLChans[ch].Users) == 0 {
			continue
		}

		for loc, data := range m.ADLChans[ch].Users {
			if (midCh < 0 || int(loc.MidCh) == midCh) && data.Sustained&kind != 0 {
				data.Sustained &^= kind

				if data.Sustained == HoldNone {
					delete(m.ADLChans[ch].Users, loc)
				} else {
					m.ADLChans[ch].Users[loc] = data
				}
			}
		}

		if len(m.ADLChans[ch].Users) == 0 {
			m.OPL.NoteOff(ch)
		}
	}
}

// MarkSostenutoNotes holds the notes currently sounding on a MIDI channel.
func (m *MIDIDecoder) MarkSostenutoNotes(midCh int) {
	for ch := range m.OPL.ChanCount() {
		for loc, data := range m.ADLChans[ch].Users {
			if int(loc.MidCh) == midCh && data.Sustained == HoldNone {
				data.Sustained |= HoldSostenuto
				m.ADLChans[ch].Users[loc] = data
			}
		}
	}
}

// SetRPN sets an RPN parameter
func (m *MIDIDecoder) SetRPN(midCh, value uint32, msb bool) {
	nrpn := m.chans[midCh].NRPN
	addr := uint32(m.chans[midCh].LastMRPN)*0x100 + uint32(m.chans[midCh].LastLRPN)
	key := addr

	if nrpn {
		key += 0x10000
	}

	if msb {
		key += 0x20000
	}

	ch := &m.chans[midCh]

	switch key {
	case 0x0000 + 0*0x10000 + 0*0x20000: // Pitch-bender sensitivity
		ch.Pitch.LSB = int(value)
	case 0x0000 + 0*0x10000 + 1*0x20000: // Pitch-bender sensitivity
		ch.Pitch.MSB = int(value)
	case 0x0108 + 1*0x10000 + 1*0x20000: // Vibrato speed
		if value == 64 {
			ch.Vibrato.Speed = 1.0
		} else if value < 100 {
			v := value
			if v == 0 {
				v = 1
			}
			ch.Vibrato.Speed = 1.0 / (1.6e-2 * float64(v))
		} else {
			ch.Vibrato.Speed = 1.0 / (0.051153846*float64(value) - 3.4965385)
		}
		ch.Vibrato.Speed *= 2 * math.Pi * 5.0
	case 0x0109 + 1*0x10000 + 1*0x20000: // Vibrato depth
		ch.Vibrato.Depth = (float64(value) - 64) * 0.15 * 0.01
	case 0x010A + 1*0x10000 + 1*0x20000: // Vibrato delay
		if value != 0 {
			ch.Vibrato.Delay = time.Duration(0.2092 * math.Exp(0.0795*float64(value)) * float64(time.Second))
		} else {
			ch.Vibrato.Delay = 0
		}
	}
}

// UpdatePortamento updates portamento
func (m *MIDIDecoder) UpdatePortamento(midCh uint32) {
	p := &m.chans[midCh].Portamento

	p.Rate = math.Inf(1)

	if p.Enabled && p.Level > 0 {
		p.Rate = 350.0 * math.Pow(2, -0.062*(1.0/128)*float64(p.Level))
	}
}

// UpdateGlide slides the gliding notes of all channels towards their tone.
func (m *MIDIDecoder) UpdateGlide(amount time.Duration) {
	sec := amount.Seconds()

	for a := range m.chans {
		for note, info := range m.chans[a].ActiveNotes {
			if math.IsInf(info.GlideRate, 1) {
				continue
			}

			final, prev := float64(info.Tone), info.Glide

			next := prev - sec*info.GlideRate
			if prev < final {
				next = min(prev+sec*info.GlideRate, final)
			} else {
				next = max(next, final)
			}

			if int64(next*1e6) != int64(prev*1e6) {
				info.Glide = next
				m.NoteUpdate(uint32(a), note, UpdPitch, -1)
			}
		}
	}
}

// NoteUpdateAll updates all notes on a channel
func (m *MIDIDecoder) NoteUpdateAll(midCh uint32, propsMask uint32) {
	for note := range m.chans[midCh].ActiveNotes {
		m.NoteUpdate(midCh, note, propsMask, -1)
	}
}

// NoteOff turns off a note
func (m *MIDIDecoder) NoteOff(midCh uint32, note uint8) {
	if _, ok := m.chans[midCh].ActiveNotes[note]; ok {
		m.NoteUpdate(midCh, note, UpdOff, -1)
	}
}

// UpdateVibrato updates vibrato for all channels
func (m *MIDIDecoder) UpdateVibrato(amount time.Duration) {
	sec := amount.Seconds()

	for a := range m.chans {
		ch := &m.chans[a]

		if ch.HasVibrato() && len(ch.ActiveNotes) > 0 {
			m.NoteUpdateAll(uint32(a), UpdPitch)
			ch.Vibrato.Pos += sec * ch.Vibrato.Speed
		} else {
			ch.Vibrato.Pos = 0.0
		}
	}
}

// UpdateArpeggio updates arpeggio for all channels
func (m *MIDIDecoder) UpdateArpeggio(_ time.Duration) {
	// Arpeggio logic
	for c := range m.OPL.ChanCount() {
		nuser := len(m.ADLChans[c].Users)
		if nuser > 1 {
			// Sort users for deterministic iteration
			var locs []Location

			for loc := range m.ADLChans[c].Users {
				locs = append(locs, loc)
			}

			sort.Slice(locs, func(i, j int) bool {
				if locs[i].MidCh != locs[j].MidCh {
					return locs[i].MidCh < locs[j].MidCh
				}

				return locs[i].Note < locs[j].Note
			})

			// Simple round-robin arpeggio
			m.arpeggio++

			invrate := 3
			switch {
			case nuser >= 4:
				invrate = 1
			case nuser >= 3:
				invrate = 2
			}

			idx := (int(m.arpeggio) / invrate) % nuser
			loc := locs[idx]
			data := m.ADLChans[c].Users[loc]

			if data.Sustained == HoldNone {
				if data.KONDecay <= 0 {
					m.NoteUpdate(uint32(loc.MidCh), loc.Note, UpdOff, c)
				} else {
					m.NoteUpdate(uint32(loc.MidCh), loc.Note, UpdPitch|UpdVolume|UpdPan, c)
				}
			}
		}
	}
}

// ChooseDevice chooses a device for a track
func (m *MIDIDecoder) ChooseDevice(name string) uint32 {
	// fmt.Fprintf(os.Stderr, "choosing device '%s'\n", name)

	if dev, ok := m.Devices[name]; ok {
		return dev
	}

	n := uint32(len(m.Devices)) * 16
	m.Devices[name] = n

	vol, bend := m.defaults()

	// Resize channels
	for len(m.chans) < int(n)+16 {
		m.chans = append(m.chans, Channel{
			Volume:     vol,
			DefVolume:  vol,
			DefBend:    bend,
			Expression: 127,
			Brightness: 127,
			Panning:    64,
			Pitch:      Pitch{MSB: int(bend)},

			Source: -1,
			Rate:   math.Inf(1),

			Speed: 2 * 3.141592653 * 5.0,
			Depth: 0.5 / 127,

			ActiveNotes: make(map[uint8]*NoteInfo),
		})
	}
	return n
}

// Dynamic instrument IDs are ordinary indices with bit 15 set.
// The low bits are the index into m.dyn.ins / m.dyn.metas.
const (
	DynamicIndexMask         uint16 = 0x7FFF
	DynamicInstrumentTag     uint16 = 0x8000
	DynamicMetaInstrumentTag uint32 = 0x4000000
)

// isDynamic reports whether id refers to a dynamically-loaded instrument.
func isDynamic(id uint16) bool { return has(id, DynamicInstrumentTag) }

// dyn returns the index into m.dyn.* for a dynamic id.
func dyn(id uint16) int { return int(id & DynamicIndexMask) }

// mkdyn encodes an index as a dynamic instrument id.
func mkdyn(idx int) uint16 { return uint16(idx) | DynamicInstrumentTag }

// GetAdlMetaID maps an instrument index (as stored in a track) to the
// meta-instrument id used by GetAdlMeta.
//
// When a dynamic bank is active, the result is a dynamic id; otherwise
// it is the meta-instrument ID selected from the derived BankDB tables.
func (m *MIDIDecoder) GetAdlMetaID(ins uint8, channel *Channel) uint16 {
	if m.adlBank == ^uint32(0) {
		return mkdyn(int(ins))
	}

	bank := int(m.adlBank)
	if len(Banks) == 0 {
		return 0
	}
	if bank < 0 || bank >= len(Banks) {
		bank = 0
	}

	patch := int(ins)
	midiBanks := Banks[bank].Melos
	if patch >= 128 {
		patch -= 128
		midiBanks = Banks[bank].Drums
	}
	if patch >= 128 || len(midiBanks) == 0 {
		return 0
	}

	selected := midiBanks[0]
	if channel != nil && channel.BankSelected {
		for _, midiBank := range midiBanks {
			if midiBank.Bank == channel.Bank {
				selected = midiBank
				break
			}
		}
	}
	return selected.Programs[patch]
}

// GetAdlMeta returns the meta-instrument for a meta id produced by
// GetAdlMetaID. Dynamic ids are resolved against the dynamic bank;
// everything else against the built-in ADLMetas table.
func (m *MIDIDecoder) GetAdlMeta(metaID uint16) ADLMeta {
	if isDynamic(metaID) {
		return m.dynamicMeta(dyn(metaID))
	}
	return m.staticMeta(int(metaID))
}

func (m *MIDIDecoder) dynamicMeta(idx int) ADLMeta {
	if idx < 0 || idx >= len(m.dyn.metas) {
		return ADLMeta{}
	}
	return m.dyn.metas[idx]
}

func (m *MIDIDecoder) staticMeta(idx int) ADLMeta {
	if idx < 0 || idx >= len(ADLMetas) {
		return ADLMeta{}
	}
	return ADLMetas[idx]
}

// GetAdlIns returns the AdLib instrument data for an instrument id.
// Dynamic ids are resolved against the dynamic bank; everything else
// against the built-in ADLIns table.
func (m *MIDIDecoder) GetAdlIns(insID uint16) ADLData {
	if isDynamic(insID) {
		return m.dynamicIns(dyn(insID))
	}
	return m.staticIns(int(insID))
}

func (m *MIDIDecoder) dynamicIns(idx int) ADLData {
	if idx < 0 || idx >= len(m.dyn.ins) {
		return ADLData{}
	}
	return m.dyn.ins[idx]
}

func (m *MIDIDecoder) staticIns(idx int) ADLData {
	if idx < 0 || idx >= len(ADLIns) {
		return ADLData{}
	}
	return ADLIns[idx]
}
