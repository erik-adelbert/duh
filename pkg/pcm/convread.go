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
	"io"
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
		err = mkError(ErrRead, err)
	}()

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
			// If there is an error, we may still have pending data to
			// return before propagating the error.
			if r.err == io.EOF && !r.flushed {
				nflush := r.conv.Flush(r.obuf)

				if nflush > 0 {
					r.pending = r.obuf[:nflush]
					r.flushed = true
					continue
				}

				r.flushed = true
			}

			// If there is no (more) pending data, propagate the error.
			if len(r.pending) == 0 {
				err = r.err
				return
			}

			// There is still pending data, continue to copy it to p.
			continue
		}

		// Read more data from the underlying reader and process it.
		var nread int
		nread, r.err = r.r.Read(r.ibuf)

		if r.err != nil && r.err != io.EOF {
			return n, r.err
		}

		if nread > 0 {
			nconv := r.conv.Process(r.obuf, r.ibuf[:nread])

			if nconv > 0 {
				r.pending = r.obuf[:nconv]
			}
		}

		// If we have reached EOF on the underlying reader,
		// flush any remaining data.
		if r.err == io.EOF && !r.flushed {
			nflush := r.conv.Flush(r.obuf)
			if nflush > 0 {
				r.pending = r.obuf[:nflush]
			}
			r.flushed = true
		}

		// If we have reached EOF and there is no pending data, return EOF.
		if r.err == io.EOF && len(r.pending) == 0 {
			err = io.EOF

			return
		}

		// Continue reading and processing data until we have filled p or reached EOF.
	}

	// Return the number of bytes read and any error encountered.
	return
}

// ReadAt reads converted PCM beginning at a source PCM byte offset. The offset
// is aligned to an input frame boundary and does not affect sequential reads.
func (r *ConvReader) ReadAt(p []byte, off int64) (n int, err error) {
	if r.ra == nil {
		return 0, ErrESPIPE
	}

	return r.ra.ReadAt(p, off)
}

// remap converts a byte offset from one PCM format to another
func remap(off, from, to int64) int64 {
	if from <= 0 || to <= 0 {
		return off
	}

	x := off * to

	q := x / from
	if x%from != 0 && x < 0 { // floor instead of truncate
		q--
	}

	return q
}

// Seek sets the source PCM byte offset for the next Read operation. The offset
// is aligned to an input frame boundary and conversion state is reset.
func (r *ConvReader) Seek(offset int64, whence int) (off int64, err error) {
	if r.rs == nil {
		return 0, ErrESPIPE
	}

	defer func() {
		err = mkError(ErrSeek, err)
	}()

	ifmt := r.conv.Ifmt()
	ofmt := r.conv.Ofmt()

	ifsz := int64(ifmt.frameSize)
	ofsz := int64(ofmt.frameSize)

	iff := remap(offset, ofsz, ifsz) // map to input offset

	iff, err = r.rs.Seek(iff, whence)

	if err != nil {
		return
	}

	off = remap(iff, ifsz, ofsz) // map back to output offset

	// Reset the conversion state and clear any pending output
	r.pending = nil
	r.err = nil
	r.flushed = false

	r.conv.Reset()

	return
}

const kB = 1024
