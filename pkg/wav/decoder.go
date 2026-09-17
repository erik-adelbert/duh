// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Package wav implements a WAV file reader.

package wav

import (
	"fmt"
	"io"
	"slices"
	"time"
)

// Decoder is a WAV file reader that implements the PCMStream interface.
type Decoder struct {
	format Format

	r  io.Reader
	rs io.ReadSeeker

	doff, dsz, off int64 // data offset, data size, current read offset
}

// Decode creates a new Reader for the given io.Reader and name.
func Decode(in io.Reader) (r *Decoder, err error) {
	readErr := func(ctx any) error {
		return mkError(ErrDecode, ctx)
	}

	defer func() {
		err = readErr(err) // wrap any read error with context
	}()

	const sigSize = 12
	var riff [sigSize]byte

	_, err = io.ReadFull(in, riff[:])

	if err != nil {
		return
	}

	if string(riff[0:4]) != "RIFF" || string(riff[8:12]) != "WAVE" {
		err = readErr("not a RIFF/WAV file")
		return
	}

	var (
		format Format

		foff int64 = sigSize // file offset

		hasfmt  bool
		hasdata bool
	)

	var hdr [8]byte

	for {
		_, err = io.ReadFull(in, hdr[:])

		if err != nil {
			return
		}

		foff += 8

		chunkID := string(hdr[0:4])
		sz := get32(hdr[4:])

		switch chunkID {
		case "fmt ": // final space is intended
			switch {
			case hasfmt:
				err = readErr("duplicate format chunk")
				return

			case sz < 16 || sz > 40:
				err = readErr("bad format chunk size")
				return
			}

			format, err = getfmt(in, int(sz))

			if err != nil {
				return
			}

			foff += int64(sz)

			// skip padding if present
			if sz&1 == 1 {
				err = skipBytes(in, 1)
				foff++
			}

			if err != nil {
				return
			}

			hasfmt = true
		case "data":
			if hasdata {
				err = readErr("duplicate data chunk")
				return
			}

			if !hasfmt {
				err = readErr("missing format chunk")
				return
			}

			rs, _ := in.(io.ReadSeeker)

			return &Decoder{
				r:      io.LimitReader(in, int64(sz)+1),
				rs:     rs,
				format: format,
				doff:   foff,
				dsz:    int64(sz),
			}, nil
		default:
			// Skip unknown chunk
			nskip := int64(sz)

			if nskip&1 == 1 { // chunks are word-aligned
				nskip++
			}

			err = skipBytes(in, nskip)

			if err != nil {
				return
			}

			foff += nskip
		}
	}
}

// Read reads bytes from the WAV data into the provided buffer.
func (r *Decoder) Read(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrDecode, err)
	}()

	if r.off >= r.dsz {
		return 0, io.EOF
	}

	limit := min(int64(len(p)), r.dsz-r.off)
	p = p[:limit]

	n, err = r.r.Read(p)
	r.off += int64(n)

	return
}

// ReadAt reads bytes from the WAV data at the given offset into the provided buffer.
func (r *Decoder) ReadAt(p []byte, off int64) (n int, err error) {
	if r.rs == nil {
		err = mkError(ErrESPIPE, "illegal seek")
		return
	}

	defer func() {
		err = mkError(ErrDecode, err)
	}()

	// Validate offset and buffer
	switch {
	case off < 0:
		err = mkError(ErrSeek, "negative offset")
		return
	case off > r.dsz:
		err = io.EOF
		return
	case len(p) == 0:
		// nil read, return immediately
		return
	}

	// Save the current read position to restore it later.
	off0, err := r.rs.Seek(0, io.SeekCurrent)

	if err != nil {
		return
	}

	defer func() {
		_, err = r.rs.Seek(off0, io.SeekStart)
	}()

	// Adjust the buffer size if it exceeds the remaining data
	nwant := len(p)
	if nwant > int(r.dsz-off) {
		p = p[:r.dsz-off]
	}

	_, err = r.rs.Seek(r.doff+off, io.SeekStart)

	if err != nil {
		return
	}

	n, err = r.rs.Read(p)

	if n < nwant && err == nil {
		// If fewer bytes were read than requested, it indicates EOF.
		err = io.EOF
	}

	return
}

// Reset resets the reader to the beginning of the WAV data.
func (r *Decoder) Reset() (err error) {
	if r.rs == nil {
		err = mkError(ErrESPIPE, "illegal seek")
		return
	}

	_, err = r.rs.Seek(r.doff, io.SeekStart)

	if err != nil {
		return mkError(ErrSeek, err)
	}

	r.off = 0

	return
}

// Seek sets the read offset for the next Read operation.
func (r *Decoder) Seek(offset int64, whence int) (n int64, err error) {
	var dst int64

	errSeek := func(ctx any) error {
		return mkError(ErrSeek, ctx)
	}

	dst = offset

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		dst += r.off
	case io.SeekEnd:
		dst += r.dsz
	default:
		err = errSeek("invalid whence")
		return
	}

	if dst < 0 {
		err = errSeek("negative offset")
		return
	}

	// Align to frame boundary
	fsz := int64(r.format.ChannelCount() * (r.format.BitDepth() / 8))

	if fsz == 0 {
		err = errSeek("invalid frame size")
		return
	}

	dst = min((dst/fsz)*fsz, r.dsz)

	_, err = r.rs.Seek(dst+r.doff, io.SeekStart)

	if err != nil {
		err = errSeek(err)
		return
	}

	r.off = dst

	return r.off, nil
}

func (r *Decoder) Format() Format {
	return r.format
}

func (r *Decoder) Size() int64 {
	return r.dsz
}

func (r *Decoder) Duration() time.Duration {
	return r.format.Duration(r.dsz)
}

func (r *Decoder) String() string {
	dsz := r.dsz / kB

	return fmt.Sprintf(
		"WAV{%v, %v, %dkB}",
		r.format, r.format.Duration(r.dsz), dsz,
	)
}

const (
	FormatPCM        = 0x0001
	FormatIEEEFloat  = 0x0003
	FormatExtensible = 0xFFFE
)

func getfmt(in io.Reader, sz int) (format Format, err error) {
	fmtErr := func(reason any) error {
		return mkError(errFMT, reason)
	}

	var buf [40]byte

	_, err = io.ReadFull(in, buf[:sz])

	if err != nil {
		err = mkError(errFMT, err)
		return
	}

	afmt := get16(buf[0:])

	supported := []uint16{FormatPCM, FormatIEEEFloat, FormatExtensible}

	if !slices.Contains(supported, afmt) {
		err = fmtErr("unsupported audio format")
		return
	}

	nchan := get16(buf[2:])

	if nchan == 0 || nchan > 8 {
		err = fmtErr("unsupported channel count")
		return
	}

	sr := get32(buf[4:])

	if sr == 0 || sr > 192_000 {
		err = fmtErr("unsupported sample rate")
		return
	}

	nbit := get16(buf[14:])

	if nbit == 0 || nbit > 32 || nbit&7 != 0 {
		err = fmtErr("unsupported bit depth")
		return
	}

	if afmt == FormatExtensible {
		if sz != 40 {
			err = fmtErr("bad extensible chunk")
			return
		}

		afmt = get16(buf[24:])
	}

	var tag rune

	switch afmt {
	case FormatPCM:
		tag = 's'

		if nbit == 8 {
			tag = 'u' // unsigned 8-bit PCM
		}
	case FormatIEEEFloat:
		tag = 'f'
	default:
		err = fmtErr("unsupported audio format")
		return
	}

	format, err = MkFormat(
		tag, int(nchan), int(nbit), int(sr),
	)

	return
}

func skipBytes(in io.Reader, n int64) error {
	if n <= 0 {
		return nil
	}

	_, err := io.CopyN(io.Discard, in, n)

	return err
}
