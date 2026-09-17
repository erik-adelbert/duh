package xm

import "io"

type Reader interface {
	io.ReadCloser
	io.ReaderAt
	io.Seeker
}

type XM struct {
	Header      XMHeader
	Instruments []XMInstrument
	Patterns    []XMPattern
	Orders      []byte
}

type XMHeader struct {
	Xtended        [17]byte
	Name           [20]byte
	Type           byte
	Tracker        [20]byte
	Version        uint16
	HeaderSize     uint32
	SongLength     uint16
	NumPatterns    uint16
	NumInstruments uint16
	Flags          uint16
	DefaultTempo   uint16
	DefaultSpeed   uint16
}

type XMInstrument struct {
	HeaderSize uint32
	Name       [22]byte
	Type       byte
	NumSamples uint16
	Samples    []XMSample

	XMExtraSampleHeader
}

type XMPattern struct {
	HeaderSize  uint32
	PackingType byte
	NumRows     uint16
	Size        uint16
	Data        []byte
}

type XMExtraSampleHeader struct {
	HeaderSize        uint32
	SampleNumbers     [96]byte
	VolumePoints      [48]byte
	PanningPoints     [48]byte
	VolumePointCount  byte
	PanningPointCount byte
	VolumeSustain     byte
	VolumeLoopStart   byte
	VolumeLoopEnd     byte
	PanningSustain    byte
	PanningLoopStart  byte
	PanningLoopEnd    byte
	VolumeType        byte
	PanningType       byte
	VibratoType       byte
	VibratoSweep      byte
	VibratoDepth      byte
	VibratoRate       byte
	VolumeFadeout     uint16
	Reserved          uint16
}

type XMSampleHeader struct {
	Size         uint32
	LoopStart    uint32
	LoopSize     uint32
	Volume       byte
	FineTune     int8
	Type         byte
	Panning      byte
	RelativeNote int8
	Reserved     byte
	Name         [22]byte
}

type XMSample struct {
	XMSampleHeader
}
