package s3m

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/erik-adelbert/duh/internal/help"
	"golang.org/x/exp/constraints"
)

type Reader interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type S3M struct {
	Header
	Instruments []S3MInstrument
	Patterns    []S3MPattern
	Order       []byte
	Pannings    [32]byte
}

func New(r Reader) (*S3M, error) {
	m := new(S3M)

	// header
	err := bread(r, &m.Header)

	fmt.Printf("S3M header: %+v\n%s %s\n", m.Header, asString(m.Name[:]), asString(m.Sig2[:]))

	switch {
	case err != nil:
		return nil, mkError(ErrNew, err)
	case string(m.Sig2[:]) != "SCRM":
		return nil, mkError(ErrNew, "invalid S3M signature")
	}

	// orders
	m.Order = make([]byte, m.OrderCount)

	err = bread(r, &m.Order)

	if err != nil {
		return nil, mkError(ErrNew, err)
	}

	fmt.Printf("S3M order: %v\n", m.Order)

	// instrument pointer table
	instrPtrs := make([]pptr16, m.InstrCount)

	err = bread(r, instrPtrs)

	if err != nil {
		return nil, mkError(ErrNew, err)
	}

	fmt.Printf("S3M instrument pointers: %v\n", instrPtrs)

	// pattern pointer table
	patPtrs := make([]pptr16, m.PatternCount)

	err = bread(r, &patPtrs)

	if err != nil {
		return nil, mkError(ErrNew, err)
	}

	fmt.Printf("S3M pattern pointers: %v\n", patPtrs)

	if m.Panning == customPanning {
		err = bread(r, &m.Pannings)

		if err != nil {
			return nil, mkError(ErrNew, err)
		}

		fmt.Printf("S3M custom panning: %v\n", m.Pannings)
	}

	// instruments
	const (
		kindTitle uint8 = iota
		kindPCM
		kindAdlib
	)

	m.Instruments = make([]S3MInstrument, len(instrPtrs))

	var id InstrumentID

	for i, ptr := range instrPtrs {
		fmt.Printf("loading instrument %d at offset %d\n", i, ptr.off())

		err = breadAt(r, &id, ptr)

		if err != nil {
			return nil, mkError(ErrNew, err)
		}

		fmt.Printf("instrument: kind = %d, name = %s\n", id.Kind, asString(id.Filename[:]))

		switch id.Kind {
		case kindTitle:
			var title TitleHeader

			err = breadAt(r, &title, ptr)

			if err != nil {
				return nil, mkError(ErrNew, err)
			}

			m.Instruments[i] = S3MInstrument{InstrumentID: id, InstrumentHeader: title}
		case kindPCM:
			fmt.Printf("loading PCM instrument %d at offset %d\n", i, ptr.off())
			var pcm PCMHeader

			err = breadAt(r, &pcm, ptr)

			if err != nil {
				return nil, mkError(ErrNew, err)
			}

			// name := pcm.Name[:]
			// fmt.Printf("PCM header: name = %v, sig = %s\n", name, pcm.Sig[:])
			fmt.Printf("PCM header: %+v\n", pcm)

			if pcm.Length == 0 {
				continue // skip empty samples
			}

			instr := S3MInstrument{InstrumentID: id, InstrumentHeader: pcm}

			off, size := pcm.Ptr24.off(), int(pcm.Length)

			if pcm.Flags.IsStereo() {
				size *= 2
			}

			if pcm.Flags.Is16Bit() {
				size *= 2
			}

			if pcm.Packing == 4 { // ADPCM4
				size -= 16             // 16 bytes for the compression table
				size = 16 + (size+1)/2 // each byte becomes 2 samples, add 16 bytes for the table
			}

			instr.Sample = make([]byte, size)

			fmt.Printf("reading sample %d at offset %d, size = %d\n", i, off, size)

			n, err := readAt(r, instr.Sample, off)

			switch {
			case n != size:
				return nil, mkError(ErrNew, io.ErrUnexpectedEOF)
			case err != nil && err != io.EOF:
				return nil, mkError(ErrNew, err)
			}

			// instr.PCM.Length = uint32(n)
			m.Instruments[i] = instr
		case kindAdlib:
			fmt.Printf("loading Adlib instrument %d at offset %d\n", i, ptr.off())
			var adlib AdlibHeader

			err = breadAt(r, &adlib, ptr)

			if err != nil {
				return nil, mkError(ErrNew, err)
			}

			m.Instruments[i] = S3MInstrument{InstrumentID: id, InstrumentHeader: adlib}
		}
	}

	// patterns
	fmt.Printf("loading %d patterns\n", len(patPtrs))
	m.Patterns = make([]S3MPattern, len(patPtrs))

	for i, ptr := range patPtrs {
		fmt.Printf("loading pattern %d at offset %d\n", i, ptr.off())
		if ptr == 0 {
			continue
		}

		var size uint16

		err = breadAt(r, &size, ptr)

		if err != nil {
			return nil, mkError(ErrNew, err)
		}

		size -= 2 // size includes the 2 bytes of the size field itself
		packed := make([]byte, size)

		off := ptr.off() + 2 // skip the size field

		_, err := readAt(r, packed, off)

		if err != nil {
			return nil, mkError(ErrNew, err)
		}

		m.Patterns[i] = unpackPattern(packed)
	}

	return m, nil
}

func (s3m *S3M) String() string {
	return fmt.Sprintf(
		"S3M: %s, %d, %d",
		asString(s3m.Name[:]), s3m.Creator, s3m.Format,
	)
}

const (
	MinSpeed       = 6
	MaxSpeed       = 0x1F
	MinTempo       = 40
	MaxTempo       = 240
	MinVolume      = 0
	MaxVolume      = 256
	VolumeMask     = 0x7F
	customPanning  = 0xFC
	st320          = 0x1320 // Scream Tracker 3.20
	st3AmigaLimits = 0x10
	st3VolSlides   = 0x40
	unusedCh       = 0xFF
)

type Header struct {
	Name         [28]byte  // 28 Song name
	Sig1         uint8     // 1 always 0x1A
	Type         uint8     // 1 ST3 always 0x10
	_            uint16    // 2 reserved1
	OrderCount   uint16    // 2
	InstrCount   uint16    // 2
	PatternCount uint16    // 2
	Flags        uint16    // 2
	Creator      uint16    // 2
	Format       uint16    // 2
	Sig2         [4]byte   // 4 always "SCRM"
	GlobalVolume uint8     // 1
	DefaultSpeed uint8     // 1
	DefaultTempo uint8     // 1
	SongPreAmp   uint8     // 1
	_            uint8     // 1 ultraClick
	Panning      uint8     // 1
	_            [8]byte   // 8 reserved2
	SpecialPtr   uint16    // 2
	ChanSettings [32]uint8 // 32
}

type sampleType uint16

const (
	samplePCM8S sampleType = iota
	samplePCM8U
	samplePCM8D
	sampleADPCM4
	samplePCM16D
	samplePCM16S
	samplePCM16U
	samplePCM16M
	sample16Bit    sampleType = 0x04
	sampleStereo   sampleType = 0x08
	sampleSTPCM8S  sampleType = samplePCM8S | sampleStereo
	sampleSTPCM8U  sampleType = samplePCM8U | sampleStereo
	sampleSTPCM8D  sampleType = samplePCM8D | sampleStereo
	sampleSTPCM16S sampleType = samplePCM16S | sampleStereo
	sampleSTPCM16U sampleType = samplePCM16U | sampleStereo
	sampleSTPCM16D sampleType = samplePCM16D | sampleStereo
	sampleSTPCM16M sampleType = samplePCM16M | sampleStereo
)

func (s sampleType) Is16Bit() bool {
	return has(s, sample16Bit)
}

func (s sampleType) IsStereo() bool {
	return has(s, sampleStereo)
}

type S3MInstrument struct {
	InstrumentID
	InstrumentHeader any
	Sample           []byte // only for PCM
}

func (s3i *S3MInstrument) AsSampleFrom(version uint16) *Sample {
	pcm, ok := s3i.InstrumentHeader.(PCMHeader)
	if !ok {
		return nil
	}

	kind := Kind(pcm.Kind)

	data, flags := toPCM8S(s3i.Sample, extractFormat(pcm, version))

	length := min(pcm.Length, MaxSampleLength)

	flags, loop := parseLoop(pcm, length, flags)

	if has(kind, KindMod|KindS3M) && !has(flags, SampleStereo|Sample16Bit) {
		fixSampleTail(data, length, flags, loop)
	}

	data = addSamplePadding(data, length, flags, loop, kind == KindS3M)

	return &Sample{
		Data:      data,
		Name:      asString(pcm.Name[:]),
		Length:    int(length),
		Loop:      loop,
		Pan:       0x80, // center
		Vol:       help.Clamp(uint16(pcm.Volume), 0, 64) << 2,
		GlobalVol: 64,
		Flags:     flags,
		C4Speed:   help.Clamp(uint(pcm.C4Speed), 1024, 8363),
	}
}

// addSamplePadding adds necessary padding and fixes loop transitions for audio interpolation
func addSamplePadding(data []int8, length uint32, flags SampleFlags, loop span, isS3M bool) []int8 {
	padding := make([]int8, 8, 16)

	i, j := loop.Start, loop.End

	if has(flags, Sample16Bit) {
		// double size for 16-bit samples
		padding = append(padding, padding...)

		i *= 2
		j *= 2
	}

	// add padding after the sample data
	data = append(data, padding...)

	if has(flags, SampleStereo) {
		data = append(data, padding...) // extra space for right channel
	}

	// fix simple looped mono samples that are too short or missing the extra samples
	// check if the Looped flag is the only loop flag set shift right by 2 to ignore
	// stereo/16-bit bits
	if has(flags, SampleLooped) && (flags>>2)&^(SampleLooped>>2) == 0 {
		if loop.End+3 > int(length) || isS3M {
			copy(data[j:], data[i:j+len(padding)])
		}
	}

	return data
}

// extractFormat determines the sample format from the PCM header and module version
func extractFormat(pcm PCMHeader, version uint16) sampleType {
	format := samplePCM8U // default format

	if version == 1 {
		format = samplePCM8S
	}

	if pcm.Flags.Is16Bit() {
		format |= sample16Bit
	}

	if pcm.Flags.IsStereo() {
		format |= sampleStereo
	}

	if pcm.Packing == 4 {
		format = sampleADPCM4
	}

	return format
}

// parseLoop extracts and validates the loop points from the PCM header
func parseLoop(pcm PCMHeader, length uint32, flags SampleFlags) (SampleFlags, span) {
	parsePoint := func(v, maxi uint32) int {
		v = min(v, maxi)

		if v < 4 {
			return 0
		}

		return int(v)
	}

	start := parsePoint(pcm.LoopStart, pcm.Length-1) // 0 is disabling

	end := parsePoint(pcm.LoopEnd, pcm.Length) // 0 is disabling
	end = min(end, int(length))                // clamp to length limit

	if !pcm.Flags.HasLoop() || start == 0 || end-start < 8 {
		flags &^= SampleLooped

		return flags, span{} // no loop
	}

	flags |= SampleLooped
	loop := span{Start: start - 1, End: end - 1} // convert to 0-indexed

	if loop.Start >= loop.End { // invalid loop
		flags &^= SampleLooped
		loop = span{}
	}

	return flags, loop
}

// fixSampleTail fixes problematic mono sample endings that can cause audio clicks
func fixSampleTail(data []int8, length uint32, flags SampleFlags, loop span) {
	if length <= 256 {
		return
	}

	smpEnd := data[length-1]
	smpFix := int8(0)

	var i int

	for i = int(length - 1); i > 1; i-- {
		smpFix = data[i]

		if smpFix != smpEnd {
			break
		}
	}

	δ := abs(int(smpFix) - int(smpEnd))

	if (!has(flags, SampleLooped) || i > loop.End) && δ > 8 {
		for i < int(length) {
			if i&7 == 0 {
				if smpFix > 0 {
					smpFix--
				}

				if smpFix < 0 {
					smpFix++
				}
			}

			data[i] = smpFix

			i++
		}
	}
}

func toPCM8S(src []byte, format sampleType) (data []int8, flags SampleFlags) {
	if len(src) < 4 {
		return
	}

	switch format {
	case samplePCM8S, samplePCM8U, samplePCM8D:
		var δ int

		if format == samplePCM8U {
			δ = -128
		}

		dataLen := min(len(src), MaxSampleLength)
		data = make([]int8, dataLen)

		for i := range data {
			v := int8(int(src[i]) + δ)

			if format == samplePCM8D {
				δ = int(v)
			}

			data[i] = v
		}
	case sampleADPCM4:
		if len(src) < 16 {
			return
		}

		var δ int8

		table, src := src[:16], src[16:] // first 16 bytes are the compression table
		dataLen := min(len(src)*2, MaxSampleLength)
		src = src[:dataLen/2]        // enforce source length
		data = make([]int8, dataLen) // each byte becomes 2 samples

		for i, b := range src {
			lo, hi := b&0x0F, b>>4

			δ += int8(table[lo])
			data[i*2] = δ

			δ += int8(table[hi])
			data[i*2+1] = δ
		}
	case samplePCM16S, samplePCM16U, samplePCM16D, samplePCM16M:
		flags |= Sample16Bit

		var δ int

		if format == samplePCM16U {
			δ = -32768
		}

		size := min(len(src), MaxSampleLength)
		data = make([]int8, size)

		for i := 0; i < size; i += 2 {
			raw := binary.LittleEndian.Uint16(src[i:])

			if format == sampleSTPCM16M {
				raw = binary.BigEndian.Uint16(src[i:])
			}

			v := int16(int(raw) + δ)

			if format == samplePCM16D {
				δ = int(v)
			}

			data[i] = int8(v & 0xFF) // low byte, we don't want to use unsafe
			data[i+1] = int8(v >> 8) // high byte
		}
	case sampleSTPCM8S, sampleSTPCM8U, sampleSTPCM8D:
		flags |= SampleStereo
		δl, δr := 0, 0

		if format == sampleSTPCM8U {
			δl, δr = -128, -128
		}

		left, right := src[:len(src)/2], src[len(src)/2:]
		size := min(len(src), MaxSampleLength)
		data = make([]int8, size)

		for i := range size / 2 {
			vL := int8(int(left[i]) + δl)
			vR := int8(int(right[i]) + δr)

			if format == sampleSTPCM8D {
				δl = int(vL)
				δr = int(vR)
			}

			data[2*i] = int8(vL)
			data[2*i+1] = int8(vR)
		}
	case sampleSTPCM16S, sampleSTPCM16U, sampleSTPCM16D, sampleSTPCM16M:
		flags |= Sample16Bit | SampleStereo
		δl, δr := 0, 0

		if format == sampleSTPCM16U {
			δl, δr = -32768, -32768
		}

		left, right := src[:len(src)/2], src[len(src)/2:]
		size := min(len(src), MaxSampleLength)
		data = make([]int8, size)

		for i := 0; i < size/2; i += 2 {
			rawL := binary.LittleEndian.Uint16(left[i:])
			rawR := binary.LittleEndian.Uint16(right[i:])

			if format == sampleSTPCM16M {
				rawL = binary.BigEndian.Uint16(left[i:])
				rawR = binary.BigEndian.Uint16(right[i:])
			}

			vL := int16(int(rawL) + δl) // [0,65535] to [-32768,32767]
			vR := int16(int(rawR) + δr)

			if format == sampleSTPCM16D {
				δl = int(vL)
				δr = int(vR)
			}

			data[2*i+0] = int8(vL & 0xFF) // left low byte
			data[2*i+1] = int8(vL >> 8)   // left high byte
			data[2*i+2] = int8(vR & 0xFF) // right low byte
			data[2*i+3] = int8(vR >> 8)   // right high byte
		}
	}

	return
}

func asString(b []byte) string {
	return string(bytes.TrimRight(b, "\x00"))
}

type S3MPattern = [][]S3MEvent

type S3MEvent struct {
	ChanID  uint8
	InstrID uint8
	Note    uint8
	Vol     uint8
	Cmd     uint16
}

type InstrumentID struct {
	Kind     uint8    // 1
	Filename [12]byte // 12
}

type TitleHeader struct {
	InstrumentID
	_       [19]byte // 19 reserved1
	Volume  byte     // 1
	_       [3]byte  // 3 reserved2
	C4Speed uint32   // 4
	_       [12]byte // 12 reserved3
	Name    [28]byte // 28
	Sig     [4]byte  // 4 "SCRT"
}

type PCMHeader struct {
	InstrumentID
	Ptr24     pptr24   // 3
	Length    uint32   // 4
	LoopStart uint32   // 4
	LoopEnd   uint32   // 4
	Volume    uint8    // 1
	_         uint8    // 1 reserved1
	Packing   uint8    // 1 packing
	Flags     PCMFlags // 1
	C4Speed   uint32   // 4
	_         [12]byte // 12 internal
	Name      [28]byte // 28
	Sig       [4]byte  // 4 "SCRS"
}

type PCMFlags uint8

const (
	PCM16Bit  PCMFlags = 0x04
	PCMStereo PCMFlags = 0x08
	PCMLooped PCMFlags = 0x01
)

func (f PCMFlags) Is16Bit() bool {
	return f&PCM16Bit != 0
}

func (f PCMFlags) IsStereo() bool {
	return f&PCMStereo != 0
}

func (f PCMFlags) HasLoop() bool {
	return f&PCMLooped != 0
}

type AdlibHeader struct {
	InstrumentID
	_       [3]byte // reserved1
	OPL2    [12]byte
	Volume  uint8
	_       uint8  // undocumented disk field
	_       uint16 // reserved2
	C2Speed uint32
	_       [12]byte // reserved3
	Name    [28]byte
	Sig     [4]byte // "SCRI"
}

// func clamp[T cmp.Ordered](v, mini, maxi T) T {
// 	// sort
// 	if mini > maxi {
// 		mini, maxi = maxi, mini
// 	}

// 	// clamp v to the [mini, maxi]
// 	return max(min(v, maxi), mini)
// }

type pptr16 uint16

func (p pptr16) off() int64 {
	return int64(p) * 16
}

type pptr24 struct {
	Hi uint8
	Lo pptr16
}

func (p pptr24) off() int64 {
	return int64(p.Hi)<<16 | p.Lo.off()
}

// func offset[T constraints.Integer](ptr T) int64 {
// 	return int64(ptr) * 16
// }

// bread reads little-endian encoded binary data from the reader into the data structure.
func bread(r io.Reader, data any) error {
	err := binary.Read(r, binary.LittleEndian, data)

	return mkError(errRead, err)
}

// breadAt reads little-endian encoded binary data from the reader at the specified offset
// into the data structure.
func breadAt(r io.ReadSeeker, data any, ptr pptr16) error {
	err := brat(r, ptr.off(), data)

	return mkError(errRead, err)
}

func readAt(r io.ReaderAt, p []byte, off int64) (int, error) {
	n, err := r.ReadAt(p, off)

	return n, mkError(errRead, err)
}

// brat reads little-endian encoded binary data from the reader at the specified offset
// into the data structure.
func brat(r io.ReadSeeker, off int64, data any) (err0 error) {
	off0, err0 := r.Seek(0, io.SeekCurrent)

	if err0 != nil {
		// seek error, return early
		return mkError(errSeek, err0)
	}

	// try everything and join errors together:

	_, err0 = r.Seek(off, io.SeekStart)

	err1 := bread(r, data)

	_, err2 := r.Seek(off0, io.SeekStart)

	err0 = joinError(err0, err1, err2)

	return mkError(errSeek, err0)
}

// has checks if the specified flag is set in the given integer value.
func has[T constraints.Integer](v T, flag T) bool {
	return (v & flag) != 0
}

// unpackPattern unpacks the compressed pattern data into a matrix of S3MEvent.
func unpackPattern(data []byte) S3MPattern {
	const (
		noteFlag byte = 0x20
		volFlag  byte = 0x40
		cmdFlag  byte = 0x80
	)

	pats := make(S3MPattern, 64)
	for i := range pats {
		pats[i] = make([]S3MEvent, 0, 16)
	}

	j := 0 // index into the packed data
PatScan:
	for i := range pats {
		for j < len(data) {
			flags := data[j]

			j++

			if flags == 0 {
				continue PatScan // end of pattern row
			}

			// parse the event data based on the flags
			var e S3MEvent

			e.ChanID = flags & 0x1F

			if has(flags, noteFlag) {
				if j < len(data)-1 && data[j] != 0 {
					e.Note = data[j]
					e.InstrID = data[j+1]
				}

				j += 2
			}

			if has(flags, volFlag) {
				if j < len(data) {
					e.Vol = data[j]
				}

				j++
			}

			if has(flags, cmdFlag) {
				if j < len(data)-1 && data[j] != 0 {
					e.Cmd = uint16(data[j]-1)<<8 | uint16(data[j+1])
				}

				j += 2
			}

			pats[i] = append(pats[i], e)
		}
	}

	return pats
}

func abs(n int) int {
	if n < 0 {
		return -n
	}

	return n
}
