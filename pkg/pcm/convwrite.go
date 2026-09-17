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

// ConvWriter converts PCM data using a Converter before writing it to the underlying writer.
type ConvWriter struct {
	out io.Writer

	conv *Converter

	buf []byte
}

func NewConvWriter(w io.Writer, conv *Converter) *ConvWriter {
	var bufsz = 64 * kB

	if conv.ifmt.bitDepth == 64 || conv.ofmt.bitDepth == 64 {
		bufsz = 128 * kB
	}

	return &ConvWriter{
		out:  w,
		conv: conv,
		buf:  make([]byte, max(bufsz)),
	}
}

func (w *ConvWriter) Write(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrWrite, err)
	}()

	sz := w.conv.Process(w.buf, p)

	if sz == 0 {
		return len(p), nil
	}

	_, err = w.out.Write(w.buf[:sz])

	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (w *ConvWriter) Close() (err error) {
	nflush := w.conv.Flush(w.buf)

	if nflush > 0 {
		_, err := w.out.Write(w.buf[:nflush])

		if err != nil {
			return mkError(ErrClose, err)
		}
	}

	return
}
