// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pp20

import (
	"fmt"
	"io"
	"sync"
)

const (
	MaxCompressedSize   = 9 * MB
	MaxUncompressedSize = 16 * MB
	minCompressedSize   = 512
)

// Header represents the PP20 file header.
type Header struct {
	Name               string // file name
	CompressedSize64   int64  // bytes
	UncompressedSize64 int64  // bytes

	// internal fields
	coefs      [4]byte // efficiency bytes
	start, end int64   // data start and end positions
	skip       int     // skip bits/bytes offset
}

// Unpacker implements a PP20 file reader.
// It lazily decompresses the entire PP20 stream on first read.
type Unpacker struct {
	Header

	once      sync.Once // ensures decrunching happens only once
	errUnpack error     // stores any error during unpacking

	src io.ReaderAt // underlying reader
	x   []byte      // internal decompression buffer
	off int64       // current read offset in decompressed data
}

// Unpack creates a new PP20 decompressor from an io.ReaderAt and size.
func Unpack(ra io.ReaderAt, size int64) (*Unpacker, error) {
	r := new(Unpacker)

	err := r.init(ra, "", size)

	if err != nil {
		return nil, mkError(ErrArchive, err)
	}

	return r, nil
}

// Read reads uncompressed data into the provided buffer.
func (r *Unpacker) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		// Zero-length read, return immediately
		return
	}

	err = r.unpackOnce()

	if err != nil {
		return 0, mkError(ErrArchive, err)
	}

	end := int64(r.UncompressedSize64)

	// Return EOF if read offset is at or beyond the end of decompressed data
	if r.off >= end {
		return 0, io.EOF
	}

	// Copy data from decompressed buffer to output
	n = copy(p, r.x[r.off:])
	r.off += int64(n)

	if n < len(p) {
		// Fewer bytes were read than requested
		err = io.EOF
	}

	return
}

// ReadAt reads uncompressed data into the provided buffer at a specific offset.
func (r *Unpacker) ReadAt(p []byte, off int64) (n int, err error) {
	// Validate offset and buffer
	switch {
	case off < 0:
		return 0, mkError(ErrSeek, "negative offset")
	case len(p) == 0:
		// Zero-length read, return immediately
		return
	}

	err = r.unpackOnce()

	if err != nil {
		return 0, mkError(ErrArchive, err)
	}

	if off >= int64(len(r.x)) {
		return 0, io.EOF
	}

	n = copy(p, r.x[off:])

	if n < len(p) {
		err = io.EOF
	}

	return
}

// Reset resets the read offset to the beginning of the decompressed data.
func (r *Unpacker) Reset() {
	r.off = 0
}

// Seek sets the read offset for the next Read operation.
func (r *Unpacker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		// offset is already correct
	case io.SeekCurrent:
		offset += r.off
	case io.SeekEnd:
		offset += r.UncompressedSize64
	default:
		return 0, mkError(ErrSeek, "invalid whence")
	}

	if offset < 0 {
		return 0, mkError(ErrSeek, "negative offset")
	}

	r.off = offset

	return r.off, nil
}

// String returns a string representation of the PP20 reader.
func (r *Unpacker) String() string {
	return fmt.Sprintf(
		"PP20 Reader: %s Packed/Unpacked Size: %dB/%dB",
		r.Name, r.CompressedSize64, r.UncompressedSize64,
	)
}

// init initializes the PP20 reader with the provided io.ReaderAt and size.
func (r *Unpacker) init(src io.ReaderAt, name string, size int64) error {
	// minimal size and alignment check size should be multiple of 4
	if size < minCompressedSize || size&3 != 0 || size > MaxCompressedSize {
		return mkError(errFormat, "invalid file size")
	}

	r.CompressedSize64 = size
	r.src = src
	// Read the first 4 bytes and check for "PP20" signature
	buf := make([]byte, 4)

	// Read the first 4 bytes to check for the "PP20" signature
	if _, err := src.ReadAt(buf, 0); err != nil {
		return err
	}

	if string(buf) == "PX20" {
		// PX20 format is not supported
		return mkError(errFormat, "PX20 format not supported")
	}

	if string(buf) != "PP20" {
		return mkError(errFormat, "invalid signature")
	}

	// Read the last 4 bytes to get the uncompressed size and skip value
	if _, err := src.ReadAt(buf, size-4); err != nil {
		return err
	}

	// The last 4 bytes contain the uncompressed size (3 bytes) and skip value (1 byte)
	skip := int(buf[3])

	if skip > 32 {
		return mkError(errFormat, "invalid skip value")
	}

	r.skip = skip

	// The uncompressed size is stored in the first 3 bytes of the last 4 bytes
	fullSize := (int64(buf[0]) << 16) | (int64(buf[1]) << 8) | int64(buf[2])

	if fullSize < minCompressedSize || fullSize > MaxUncompressedSize || fullSize > 16*size {
		return mkError(errFormat, "invalid uncompressed size")
	}

	r.UncompressedSize64 = fullSize

	// Read 4 efficiency bytes from offset 4
	if _, err := src.ReadAt(buf, 4); err != nil {
		return err
	}

	for i := range buf {
		if buf[i] < 9 || buf[i] > 15 {
			return mkError(errFormat, "invalid efficiency byte")
		}
	}

	n := copy(r.coefs[:], buf)

	if n != len(r.coefs) {
		return mkError(errFormat, "failed to read efficiency bytes")
	}

	// Set data start and end positions
	r.start = 8
	r.end = size - 4

	if r.Name = name; name == "" {
		r.Name = "Unnamed"
	}

	return nil
}

// unpackOnce decompresses the PP20 data into the internal buffer on the first call.
func (r *Unpacker) unpackOnce() error {
	r.once.Do(func() {
		r.x = make([]byte, r.UncompressedSize64)
		r.errUnpack = r.unpack() // nil if successful
	})

	return r.errUnpack
}

// unpack decompresses the PP20 data into the internal buffer.
func (r *Unpacker) unpack() (err error) {
	out := r.x

	if len(out) == 0 {
		// zero-length buffer, nothing to unpack
		return nil
	}

	pksize := int(r.end - r.start)
	pkdata := make([]byte, pksize)

	n, err := r.src.ReadAt(pkdata, r.start)

	switch {
	case n != pksize:
		return mkError(errUnpack, io.ErrUnexpectedEOF)
	case err != nil && err != io.EOF:
		return mkError(errUnpack, err)
	}

	err = r.decrunch(pkdata)

	if err != nil {
		return mkError(errUnpack, err)
	}

	return nil
}

// decrunch performs the PP20 decompression algorithm.
func (r *Unpacker) decrunch(in []byte) error {
	out := r.x

	coefs := r.coefs

	bin := newBitReaderLE(in)

	_, err := bin.ReadBitsLE(r.skip)

	if err != nil {
		return mkError(errDecrunch, err)
	}

	j := len(out)

	for j > 0 {
		b, err := bin.ReadBitsLE(1)

		if err != nil {
			return mkError(errDecrunch, err)
		}

		// Literal run
		if b == 0 {
			nmatch := 1

			for {
				b, err := bin.ReadBitsLE(2)

				if err != nil {
					return mkError(errDecrunch, err)
				}

				nmatch += b

				if b != 3 {
					break
				}
			}

			// Read literal bytes
			for range nmatch {
				if j--; j < 0 {
					return nil
				}

				b, err := bin.ReadBitsLE(8)

				if err != nil {
					return mkError(errDecrunch, err)
				}

				out[j] = byte(b)
			}
		}

		// Match
		b, err = bin.ReadBitsLE(2)

		if err != nil {
			return mkError(errDecrunch, err)
		}

		// The offset width is the efficiency byte
		offBits := int(coefs[b])

		// nmatch is the match length, which is initially set to b + 2
		nmatch := b + 2

		var off int

		switch b {
		default: // regular offset
			off, err = bin.ReadBitsLE(offBits)

			if err != nil {
				return mkError(errDecrunch, err)
			}
		case 3: // extended offset

			// Read extra offset bits
			b, err := bin.ReadBitsLE(1)

			if err != nil {
				return mkError(errDecrunch, err)
			}

			// If the extra bit is 0, the offset is 7bits wide
			if b == 0 {
				offBits = 7
			}

			off, err = bin.ReadBitsLE(offBits)

			if err != nil {
				return mkError(errDecrunch, err)
			}

			for {
				b, err := bin.ReadBitsLE(3)

				if err != nil {
					return mkError(errDecrunch, err)
				}

				nmatch += b

				if b != 7 {
					break
				}
			}
		}

		// Copy match bytes from the output buffer to the current position
		for range nmatch {
			if j > 0 && j+off < len(out) {
				out[j-1] = out[j+off]
			}

			j--
		}
	}

	return nil
}

// bitReaderLE is a bit reader that reads bits in little-endian order from
// a byte slice.
type bitReaderLE struct {
	buf []byte
	idx int

	bitCnt int
	bitBuf uint64
}

// newBitReaderLE creates a new bitReaderLE for the given byte slice.
func newBitReaderLE(buf []byte) *bitReaderLE {
	br := &bitReaderLE{
		buf: buf,
		idx: len(buf) - 1, // start reading down from the last byte
	}

	if br.idx >= 0 {
		br.bitBuf = uint64(buf[br.idx])
		br.idx--
		br.bitCnt = 8
	}

	return br
}

// ReadBitsLE reads n bits in little-endian order from the bitReaderLE.
func (br *bitReaderLE) ReadBitsLE(n int) (int, error) {
	// Validate bit count
	switch {
	case n == 0:
		return 0, nil // Zero bits requested, return 0
	case n < 0:
		return 0, mkError(errBits, "negative bit count")
	case n > 63:
		return 0, mkError(errBits, "bit count too large")
	}

	// Fill the bit buffer until it has at least n bits
	for br.bitCnt < n && br.idx >= 0 {
		b := br.buf[br.idx]

		br.bitBuf |= uint64(b) << br.bitCnt
		br.bitCnt += 8

		br.idx--
	}

	// If there are not enough bits, inject 0s
	br.bitCnt += ((n - br.bitCnt + 7) / 8) * 8 // align to next byte boundary

	// Extract the requested bits from the bit buffer
	bits := 0

	// Extract and shift the least significant bits
	for range n {
		bits = (bits << 1) | int(br.bitBuf&1)
		br.bitBuf >>= 1
	}

	// Consume the bits
	br.bitCnt -= n

	return bits, nil
}

const MB = 1024 * 1024
