package mod

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type MOD struct {
	Header
	Patterns []Pattern
	Samples  []Sample

	importer
}

func New(r Reader) (*MOD, error) {
	m := new(MOD)

	err := bread(r, &m.Header)

	if err == nil {
		err = m.loadImporter()
	}

	if err == nil {
		err = m.readPatterns(r)
	}

	if err == nil {
		err = m.readSamples(r)
	}

	return m, mkError(ErrNew, err)
}

func (m *MOD) String() string {
	return fmt.Sprintf("%s MOD: %s", m.importer, m.Name)
}

func (m *MOD) loadImporter() error {
	var ok bool

	sig := string(m.Signature[:])

	m.importer, ok = importers[sig]

	if !ok {
		return mkError(errSignature, sig)
	}

	m.reorder(m.Order[:])

	return nil
}

func (m *MOD) readPatterns(r Reader) (err error) {

	npat := 0
	for _, order := range m.Order {
		npat = max(npat, int(order))
	}
	npat++ // Add 1 because the order is zero-based.

	m.Patterns = make([]Pattern, npat)

	for i := range m.Patterns {
		m.Patterns[i], err = m.readPattern(r)

		if err != nil {
			return mkError(ErrNew, err)
		}
	}

	return
}

func (m *MOD) readSamples(r Reader) (err error) {
	m.Samples = make([]Sample, len(m.Instruments))

	maxlen := uint16(math.MaxUint16 / 2)

	for i, inst := range m.Instruments {
		if inst.Length > maxlen {
			return mkError(errRead, "sample size too large")
		}

		size := inst.Length * 2

		m.Samples[i] = make(Sample, size)

		n, err := r.Read(m.Samples[i])

		switch {
		case n != int(size):
			return mkError(errRead, io.ErrUnexpectedEOF)
		case err != nil:
			return mkError(errRead, err)
		}
	}

	return
}

type Header struct {
	Name        [20]byte
	Instruments [31]Instrument
	Length      byte
	Restart     byte
	Order       [128]byte
	Signature   [4]byte
}

type Loop struct {
	Start  uint16
	Length uint16
}

type Instrument struct {
	Name     [22]byte
	Length   uint16
	FineTune byte
	Volume   byte
	Loop
}

func (i Instrument) String() string {
	return fmt.Sprintf("Instrument: %s", i.Name[:])
}

type Pattern [64]Row

func NewPattern(chanCount int) (p Pattern) {
	for i := range p {
		p[i] = make(Row, chanCount)
	}

	return
}

type Row []Event

type Event [4]byte

type Sample []byte

func bread(r Reader, data any) error {
	return binary.Read(r, binary.BigEndian, data)
}
