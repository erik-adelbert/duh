// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wav

import (
	"io"
	"time"
)

type Encoder struct {
	Format

	out io.Writer

	buf []byte

	headerSent bool

	n uint32
}

func Encode(w io.Writer, format Format) (*Encoder, error) {
	var bufferSize = format.BufferSize(200 * ms)

	if !isWAV(format) {
		return nil, mkError(ErrEncode, "unsupported WAV format")
	}

	return &Encoder{
		Format: format,
		out:    w,
		buf:    make([]byte, 0, bufferSize),

		headerSent: false,
	}, nil
}

// Write writes PCM data to the underlying writer, buffering as necessary.
func (w *Encoder) Write(p []byte) (n int, err error) {
	defer func() {
		err = mkError(ErrEncode, err)
	}()

	if !w.headerSent {
		_, err = w.writeHeader()

		if err != nil {
			return
		}

		w.headerSent = true
	}

	todo := p

	for len(todo) > 0 {
		nfree := cap(w.buf) - len(w.buf)

		if nfree == 0 {
			err = w.Flush()

			if err != nil {
				return
			}

			nfree = cap(w.buf)
		}

		chunksz := min(len(todo), nfree)

		w.buf = append(w.buf, todo[:chunksz]...)

		n += chunksz

		w.n += uint32(chunksz)

		todo = todo[chunksz:]
	}

	return
}

// Flush writes any buffered data to the underlying writer.
func (w *Encoder) Flush() (err error) {
	if len(w.buf) == 0 {
		return
	}

	n, err := w.out.Write(w.buf)
	if err != nil {
		return
	}

	if n < len(w.buf) {
		return io.ErrShortWrite
	}

	w.buf = w.buf[:0]
	return
}

// Close flushes final samples and patches the header size fields if the file is seekable.
func (w *Encoder) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}

	wbuf := make([]byte, 4)

	if seeker, ok := w.out.(io.WriteSeeker); ok {
		patches := []struct {
			offset int64
			value  uint32
		}{
			{4, w.n + headersz - 8}, // patch RIFF chunk size
			{40, w.n},               // patch Data subchunk size
		}

		for _, p := range patches {
			_, err := seeker.Seek(p.offset, io.SeekStart)

			if err != nil {
				return err
			}

			put32(wbuf, p.value)

			_, err = seeker.Write(wbuf)

			if err != nil {
				return err
			}
		}
	}

	return nil
}

// WAV file header

const (
	headersz = 44

	offnch = 22
	offspr = 24
	offbyr = 28
	offblk = 32
	offbit = 34
	// offdat = 40
)

// WAV file header
// 0x04: ChunkSize (to be filled in)
// 0x28: Subchunk2Size (to be filled in)
var headerTemplate = []byte{
	0x00: 'R', 'I', 'F', 'F',
	0x04: 0xff, 0xff, 0xff, 0xff,
	0x08: 'W', 'A', 'V', 'E',
	0x0c: 'f', 'm', 't', ' ',
	0x10: 0x10,
	0x14: 0x1,
	0x24: 'd', 'a', 't', 'a',
	0x28: 0xff, 0xff, 0xff, 0xff,
}

func (w *Encoder) writeHeader() (n int, err error) {
	var header = headerTemplate

	sr := uint32(w.SampleRate())
	nch := uint16(w.ChannelCount())
	nbit := uint16(w.BitDepth())
	byrate := sr * uint32(nch) * uint32(nbit) / 8
	blkalign := nch * nbit / 8

	put16(header[offnch:], nch)
	put32(header[offspr:], sr)
	put32(header[offbyr:], byrate)
	put16(header[offblk:], blkalign)
	put16(header[offbit:], nbit)

	n, err = w.out.Write(header[:])

	if err == nil && n < len(header) {
		err = io.ErrShortWrite
	}

	return
}

const (
	kB = 1024
	ms = time.Millisecond
)
