package mid

import (
	"math"
	"time"
)

type Channel struct {
	Bank

	BankSelected bool

	Patch      uint8
	Volume     uint8
	DefVolume  uint8 // volume restored by Reset
	Expression uint8
	Brightness uint8

	Vibrato

	Aftertouch     uint8
	NoteAftertouch [128]uint8
	NoteTouchInUse bool

	Panning uint8

	Percussion bool // channel plays drums besides MIDI channel 10

	Portamento

	Sustain uint8
	Soft    bool
	Pitch
	DefBend uint8 // bend sensitivity (semitones) restored by Reset
	// Bend      float64
	// BendSense float64

	LastLRPN uint8
	LastMRPN uint8
	NRPN     bool

	ActiveNotes map[uint8]*NoteInfo
}

type Pitch struct {
	Bend     float64
	MSB, LSB int
}

func (p *Pitch) Sensitivity() float64 {
	return float64((p.MSB<<7)+p.LSB) / (128 * 8192.0)
}

func (p *Pitch) Value() float64 {
	return p.Bend * p.Sensitivity()
}

type Bank struct {
	LSB, MSB uint8
}

type Vibrato struct {
	Level uint8
	Pos   float64
	Speed float64
	Depth float64
	Delay time.Duration
}

type Portamento struct {
	Enabled bool
	Level   uint16
	Source  int8    // last note played on the channel, -1 if none
	Rate    float64 // semitones per second, +Inf when disabled
}

// NoteInfo holds info about an active note
type NoteInfo struct {
	Tone int16

	Vol uint8

	// Pressure is the polyphonic aftertouch, mixed into vibrato
	Pressure uint8

	// Glide is the tone currently sounding, sliding towards Tone at
	// GlideRate semitones per second (+Inf when the note doesn't glide)
	Glide     float64
	GlideRate float64

	Instrument

	// InsMidi uint8
	// InsMeta uint16
	Type NoteType

	Phys

	TTL time.Duration

	// nglide, nxtend int
}

type Phys map[int]uint16

type NoteType int

const (
	NoteDrum NoteType = 1 << iota
	NoteSilent
	NoteExtended
)

type Instrument struct {
	Midi uint8
	Meta uint16
}

// Reset puts the channel back to its power-on controllers, like libADLMIDI's
// resetAllControllers.
func (c *Channel) Reset() {
	c.Volume = c.DefVolume
	c.Brightness = 127
	c.Panning = 64

	c.ResetControllers()
}

// ResetControllers is the CC121 subset of Reset: volume, brightness and
// panning are left alone.
func (c *Channel) ResetControllers() {
	c.Pitch.Bend = 0
	c.Pitch.MSB, c.Pitch.LSB = int(c.DefBend), 0
	c.Expression = 127
	c.Sustain = 0
	c.Soft = false
	c.Aftertouch = 0
	c.NoteAftertouch = [128]uint8{}
	c.NoteTouchInUse = false
	c.Vibrato = Vibrato{
		Speed: 2 * math.Pi * 5.0,
		Depth: 0.5 / 127,
	}
	c.Portamento = Portamento{Source: -1, Rate: math.Inf(1)}
}

// HasVibrato reports whether modulation or any aftertouch drives the vibrato.
func (c *Channel) HasVibrato() bool {
	return c.Vibrato.Level > 0 || c.Aftertouch > 0 || c.NoteTouchInUse
}

// IsDrum reports whether the channel at index midCh plays percussion.
func (c *Channel) IsDrum(midCh uint32) bool {
	return midCh&0xF == 9 || c.Percussion
}

func (c *Channel) IsXGDrum() bool {
	return c.Bank.MSB == 0x7E || c.Bank.MSB == 0x7F
}

// type ChanMode int

// const (
// 	ModeAuto ChanMode = iota
// 	ModeOffDelay
// 	ModeSameInstrument
// 	ModeAnyReleased
// )

// type ChanType int

// const (
// 	TypeRegular ChanType = iota
// 	Type4opFirst
// 	Type4opSecond
// )
