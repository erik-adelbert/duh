// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"fmt"
	"io"
	"math"

	offcalc "github.com/erik-adelbert/duh/pkg/offset"
)

// ConvReader converts PCM data read from r using conv.
type ConvReader struct {
	r  io.Reader
	rs io.ReadSeeker
	ra io.ReaderAt

	conv *Converter

	ibuf    []byte
	obuf    []byte
	pending []byte

	err     error
	flushed bool
}

// OpenConv returns a Reader that converts data from r using conv.
func OpenConv(r io.Reader, proc *Converter) *ConvReader {
	ifmt, ofmt := proc.Ifmt(), proc.Ofmt()

	ibufsz := int(ifmt.BufferSize(200 * ms))

	obufsz, _ := proc.Ratio(ibufsz)

	obufsz += int(ofmt.BufferSize(1 * ms)) // add scratch space for partial frames

	ra, _ := r.(io.ReaderAt)
	rs, _ := r.(io.ReadSeeker)

	return &ConvReader{
		r:  r,
		rs: rs,
		ra: ra,

		conv: proc,

		ibuf: make([]byte, ibufsz),
		obuf: make([]byte, obufsz),
	}
}

func (r *ConvReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return // nil read, return immediately
	}

	defer func() {
		if err != io.ErrNoProgress {
			err = mkError(ErrRead, err)
		}
	}()

	nempty := 0
	for n < len(p) {
		// If there is pending converted data, copy it to p first.
		if len(r.pending) > 0 {
			ncopy := copy(p[n:], r.pending)
			r.pending = r.pending[ncopy:]
			n += ncopy

			if n == len(p) {
				return
			}
			continue
		}

		// If there is an error from the underlying reader, handle it
		// before attempting to read more data.
		if r.err != nil {
			if r.err == io.EOF && !r.flushed {
				nflush := r.conv.Flush(r.obuf)
				r.flushed = true

				if nflush > 0 {
					r.pending = r.obuf[:nflush]
					continue
				}
			}

			err = r.err
			return
		}

		// Read more data from the underlying reader and process it.
		var nread int
		nread, r.err = r.r.Read(r.ibuf)

		if nread == 0 && r.err == nil {
			nempty++
			if nempty >= maxEmptyReads {
				err = io.ErrNoProgress
				return
			}
			continue
		}

		nempty = 0
		if nread > 0 {
			nconv := r.conv.Process(r.obuf, r.ibuf[:nread])

			if nconv > 0 {
				r.pending = r.obuf[:nconv]
			}
		}
	}

	// Return the number of bytes read and any error encountered.
	return
}

// ReadAt reads converted PCM beginning at an output byte offset without
// affecting sequential reads. It converts from the beginning of the source
// to preserve resampling and dithering state, so large offsets can be slow.
func (r *ConvReader) ReadAt(p []byte, off int64) (n int, err error) {
	if r.ra == nil {
		return 0, ErrESPIPE
	}
	if off < 0 {
		return 0, mkError(ErrRead, fmt.Sprintf("negative offset: %d", off))
	}
	if len(p) == 0 {
		return 0, nil
	}

	conv, err := NewConverter(r.conv.Ofmt(), r.conv.Ifmt())
	if err != nil {
		return 0, mkError(ErrRead, err)
	}
	reader := OpenConv(io.NewSectionReader(r.ra, 0, math.MaxInt64), conv)
	if _, err := io.CopyN(io.Discard, reader, off); err != nil {
		return 0, err
	}
	n, err = io.ReadFull(reader, p)
	if err == io.ErrUnexpectedEOF {
		err = io.EOF
	}
	return n, err
}

// Seek maps an output PCM byte offset to the source and resets conversion
// state. If the source's resulting position cannot be mapped back to an output
// offset, it returns ErrOffsetOverflow after the source has moved.
func (r *ConvReader) Seek(offset int64, whence int) (off int64, err error) {
	if r.rs == nil {
		return 0, ErrESPIPE
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %w: %w", ErrPCM, ErrSeek, err)
		}
	}()

	ifmt := r.conv.Ifmt()
	ofmt := r.conv.Ofmt()

	ifsz := int64(ifmt.frameSize)
	ofsz := int64(ofmt.frameSize)

	iff, err := offcalc.Scale(offset, ofsz, ifsz)
	if err != nil {
		return 0, err
	}

	iff, err = r.rs.Seek(iff, whence)

	if err != nil {
		return
	}

	// Reset the conversion state and clear any pending output
	r.pending = nil
	r.err = nil
	r.flushed = false

	r.conv.Reset()

	off, err = offcalc.Scale(iff, ifsz, ofsz)
	return
}

const (
	kB            = 1024
	maxEmptyReads = 100
)
