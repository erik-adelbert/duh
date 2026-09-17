// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

const (
	MaxCompressedSize = 9 * MB
	minCompressedSize = 256 // unexported
)

// Header represents the MMCMP file header.
type Header struct {
	Name               string // file name
	CompressedSize64   int64  // bytes
	UncompressedSize64 int64  // bytes
	CreatorVersion     uint16 // version number

	blkOff, blkCnt int
}

// Unpacker implements a MMCMP file reader.
// It lazily decompresses the entire MMCMP stream on first read.
type Unpacker struct {
	Header

	once      sync.Once // ensures decrunching happens only once
	errUnpack error     // stores any error during unpacking

	ra  io.ReaderAt // underlying reader
	x   []byte      // internal decompression buffer
	off int64       // current read offset in decompressed data
}

// Unpack initializes a MMCMP Reader from an io.ReaderAt and size.
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

	// Decompress the MMCMP data into memory on first read
	err = r.unpackOnce()

	if err != nil {
		return 0, mkError(ErrArchive, err)
	}

	end := r.UncompressedSize64

	// Return EOF if offset is at or beyond the end of decompressed data
	if r.off >= end {
		return 0, io.EOF
	}

	// Copy data from decompressed buffer to output
	n = copy(p, r.x[r.off:])
	r.off += int64(n)

	if n < len(p) {
		// Fewer bytes were copied than requested
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

	// Decompress the MMCMP data into memory on first read
	err = r.unpackOnce()

	if err != nil {
		return 0, mkError(ErrArchive, err)
	}

	// Return EOF if offset is at or beyond the end of decompressed data
	if off >= r.UncompressedSize64 {
		return 0, io.EOF
	}

	// Copy data from decompressed buffer to output at the specified offset
	n = copy(p, r.x[off:])

	if n < len(p) {
		// If fewer bytes were copied than requested, return EOF
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
	// var newOff int64

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

// String returns a string representation of the MMCMP reader.
func (r *Unpacker) String() string {
	return fmt.Sprintf(
		"MMCMP Reader: %s Version: %d, Packed/Unpacked Size: %dB/%dB",
		r.Name, r.CreatorVersion, r.CompressedSize64, r.UncompressedSize64,
	)
}

// section represents a section of the underlying reader with an offset and length.
type section struct {
	off int64
	len int64
}

// getSection reads a section of the underlying reader into a slice of type T.
func getSection[T any](r *Unpacker, off int64, size int) (buf []T, err error) {
	switch {
	case size < 0:
		return nil, mkError(errRead, "negative size")
	case size == 0:
		return []T{}, nil // return empty slice for zero size
	case off < 0:
		return nil, mkError(errRead, "negative offset")
	}

	var zero T

	elemSize := binary.Size(zero)

	if elemSize <= 0 {
		return nil, mkError(errRead, "invalid element size")
	}

	bufSize := int64(elemSize) * int64(size)

	if bufSize > MaxCompressedSize {
		return nil, mkError(errRead, "section too large")
	}

	if off > r.CompressedSize64 || bufSize > r.CompressedSize64-off {
		return nil, mkError(errRead, "section out of bounds")
	}

	buf = make([]T, size)

	err = r.readSection(buf, section{off: off, len: bufSize})

	return
}

// init initializes the Reader by reading and validating the MMCMP header.
func (r *Unpacker) init(src io.ReaderAt, name string, size int64) error {
	// | File Header | Main Header |
	// |   0-9       | 10-23       |

	// Read file header -- 10 bytes
	fh := new(fileHeader)

	// Calculate header length: file header (10 bytes) + main header (14 bytes)
	hlen := int64(binary.Size(fileHeader{}))
	hlen += int64(binary.Size(tableHeader{}))

	hsr := io.NewSectionReader(src, 0, hlen) // header reader

	err := bread(hsr, fh)

	if err != nil {
		return mkError(errFormat, err)
	}

	// Validate file signature and header length
	if string(fh.Sig[:]) != "ziRCONia" || fh.HdrLen != 14 {
		return mkError(errFormat, "invalid header")
	}

	// Read main header -- 14 bytes
	bth := new(tableHeader)

	err = bread(hsr, bth)

	if err != nil {
		return mkError(errFormat, err)
	}

	sec := section{
		off: int64(bth.BlkOff),     // block table offset
		len: int64(bth.BlkCnt) * 4, // block table length (4 bytes per block)
	}

	// Validate main header fields
	if bth.BlkCnt == 0 || // no blocks
		(bth.Size <= 16 || bth.Size > MaxCompressedSize) || // size limits out of range
		sec.off+sec.len > size { // block table offset out of range
		return mkError(errFormat, "invalid header")
	}

	// Populate Reader
	r.Header = Header{
		Name:               name,
		CreatorVersion:     bth.Version,
		CompressedSize64:   size,
		UncompressedSize64: int64(bth.Size),

		blkOff: int(bth.BlkOff),
		blkCnt: int(bth.BlkCnt),
	}

	// Set underlying reader
	r.ra = src

	// Set default name
	if r.Name == "" {
		r.Name = "Unnamed"
	}

	return nil
}

// unpackOnce decompresses the MMCMP data into memory on the first call.
func (r *Unpacker) unpackOnce() error {
	r.once.Do(func() {
		r.x = make([]byte, r.UncompressedSize64)

		err := r.unpack() // decompress the data into r.x

		r.errUnpack = mkError(errUnpack, err)
	})

	return r.errUnpack
}

var (
	blockSize    = int64(binary.Size(block{}))
	subBlockSize = int64(binary.Size(subBlock{}))
)

// unpack decompresses the MMCMP data into a byte slice.
// | File Header | Main Header | Block Table | Block 0 | Block 1 |
// |   0-9       | 10-23       | 24-31       | ...     | ...     |
//
// Block Table: [offset_block0, offset_block1]
// Block 0: at offset_block0
// Block 1: at offset_block1
//
// Each Block:
// Header bits 16-17: ttentry count
//
// | Block Header | Subblock Table | Translation Table             | Compressed Data ... |
// | 0-19         | 20-20+(n*8)    | 20+(n*8)+1-20+(n*8)+1+ttentry |...                  |
func (r *Unpacker) unpack() error {
	out := r.x

	if len(out) == 0 {
		// Nil read, return immediately
		return nil
	}

	// Read block offset table
	off := int64(r.blkOff) // block table offset
	bot, err := getSection[uint32](r, off, r.blkCnt)

	if err != nil {
		return mkError(errFormat, err)
	}

	pksz := r.CompressedSize64

	bhlen := blockSize    // size of block header
	blast := pksz - bhlen // last valid offset for block header

	var trans [transTableLen]byte // 8bit translation table

	// Decompress each block
	for _, o := range bot {
		boff := int64(o) // block offset

		if boff > blast {
			return mkError(errBlockTable, "block out of bounds")
		}

		block, err := r.readHeader(boff)

		if err != nil {
			return err
		}

		off := boff + bhlen // data offset

		// Read subblock table
		subs, err := getSection[subBlock](r, off, int(block.SubBlockCnt))

		if err != nil {
			return mkError(errBlock, err)
		}

		// update data offset to point to the translation table or compressed data
		off += int64(len(subs)) * subBlockSize

		if !block.isCompressed() {
			err = r.sbcopy(subs, off) // copy directly to the output buffer

			if err != nil {
				return err
			}

			continue // move to the next block
		}

		// Decompress block

		// Check if the compressed data size is within bounds
		if int64(block.CompSize) > pksz-off {
			return mkError(errBlock, "compressed data out of bounds")
		}

		pkoff := off + int64(block.TTEntries) // compressed data offset

		if pkoff < 0 || pkoff > pksz {
			return mkError(errBlock, "compressed data offset out of bounds")
		}

		pkdata := make([]byte, block.CompSize) // compressed data buffer

		var nread int
		nread, err = r.ra.ReadAt(pkdata, pkoff) // read compressed data

		switch {
		case err != nil && err != io.EOF:
			return mkError(errBlock, err)
		case uint32(nread) != block.CompSize:
			return mkError(errBlock, "short read")
		}

		width := int(block.BitLen)

		// Decompress based on bit length and format
		switch {
		case block.isBit16():
			err = r.decrunch16(pkdata, subs, width, block.isDelta(), block.isABS16())

			if err != nil {
				return mkError(errBlock, err)
			}
		case block.isBit8():
			// Read translation table
			hi := block.TTEntries

			if hi > transTableLen {
				return mkError(errBlock, "translation table too large")
			}
			if off > r.CompressedSize64 || int64(hi) > r.CompressedSize64-off {
				return mkError(errBlock, "translation table out of bounds")
			}

			_, err = r.ra.ReadAt(trans[:hi:hi], off) // read translation table

			if err != nil && err != io.EOF {
				return mkError(errBlock, err)
			}

			err = r.decrunch8(pkdata, subs, width, block.isDelta(), trans[:])

			if err != nil {
				return mkError(errBlock, err)
			}
		default:
			return mkError(errBlock, "unsupported compression format") // should never happen
		}
	}

	return nil
}

// readHeader reads a block header from the underlying reader at the specified offset.
func (r *Unpacker) readHeader(off int64) (b block, err error) {
	var (
		zero block
		hdr  [20]byte
	)

	if off < 0 || off > r.CompressedSize64-int64(len(hdr)) {
		return zero, mkError(errBlock, "block header out of bounds")
	}

	_, err = r.ra.ReadAt(hdr[:], off)

	if err != nil {
		return zero, mkError(errBlock, err)
	}

	b = block{
		FullSize:    ble.Uint32(hdr[0:4]),
		CompSize:    ble.Uint32(hdr[4:8]),
		XorCheck:    ble.Uint32(hdr[8:12]),
		SubBlockCnt: ble.Uint16(hdr[12:14]),
		Flags:       ble.Uint16(hdr[14:16]),
		TTEntries:   ble.Uint16(hdr[16:18]),
		BitLen:      ble.Uint16(hdr[18:20]),
	}

	if b.CompSize*b.FullSize == 0 || b.SubBlockCnt == 0 {
		return zero, mkError(errBlock, "invalid block sizes")
	}

	if b.TTEntries > transTableLen && b.isCompressed() && b.isBit8() {
		return zero, mkError(errBlock, "invalid translation table")
	}

	if b.CompSize <= uint32(b.TTEntries) {
		return zero, mkError(errBlock, "invalid compressed size")
	}

	sboff := 20 + int64(off)         // subblock table offset
	sbsz := int64(b.SubBlockCnt) * 8 // subblock table size

	if sboff+sbsz > r.CompressedSize64 {
		return zero, mkError(errBlock, "subblock out of bounds")
	}

	if b.isCompressed() {
		maxBitLen := uint16(8)

		if b.isBit16() {
			maxBitLen = 16
		}

		if b.BitLen > maxBitLen {
			return zero, mkError(errBlock, "invalid bit length")
		}
	}

	return b, nil
}

// readSection reads a section of the underlying reader into the provided buffer.
func (r *Unpacker) readSection(buf any, sec section) error {
	sr := io.NewSectionReader(r.ra, sec.off, sec.len) // subblock reader

	err := binary.Read(sr, ble, buf)

	if err != nil && err != io.EOF {
		return err
	}

	return nil
}

// sbcopy copies subblocks data at the given offset into the provided buffer without decompression.
func (r *Unpacker) sbcopy(subBlocks []subBlock, off int64) error {
	out := r.x

	for _, sb := range subBlocks {
		if !sb.fits(out) {
			return mkError(errBlock, "uncompressed block out of bounds")
		}

		lo, hi := sb.Pos, sb.Pos+sb.Size

		dst := out[lo:hi:hi]

		_, err := r.ra.ReadAt(dst, off)

		if err != nil {
			return mkError(errBlock, err)
		}

		off += int64(sb.Size)
	}

	return nil
}

// decrunch8 decompresses 8-bit compressed data into the provided buffer.
func (r *Unpacker) decrunch8(in []byte, subBlocks []subBlock, bitLen int, delta bool, transTable []byte) error {
	out := r.x

	br := newBitReader(in)

	dacc := 0 // delta accumulator is shared across subblocks

SubBlockScan:
	for _, sb := range subBlocks {
		if !sb.fits(out) {
			return mkError(errDecrunch, "decompressed block out of bounds")
		}

		// Get the destination slice for the current subblock
		lo, hi := sb.Pos, sb.Pos+sb.Size
		dst := out[lo:hi:hi]

		// Decompress each subblock
		i := 0
		for i < len(dst) {
			// Read symbol from bitstream
			b, err := br.readN(bitLen + 1)

			if err != nil {
				return mkError(errDecrunch, err)
			}

			sym := b

			if b >= command8[bitLen] { // command detected
				nfetch := fetch8[bitLen]
				nxtBitLen, err := br.readN(nfetch) // read additional bits for the command

				if err != nil {
					return mkError(errDecrunch, err)
				}

				nxtBitLen += (b - command8[bitLen]) << nfetch // calculate new bit length

				if nxtBitLen != bitLen { // bit length change
					bitLen = nxtBitLen & 0x07 // update bit length
					continue                  // skip to next symbol
				}

				b, err = br.readN(3) // read 3 bits for the next command

				if err != nil {
					return mkError(errDecrunch, err)
				}

				sym = 0xF8 | b // unpacked value is 0xF8 + d for this command

				if b == 0x07 { // 0x07 indicates a special case
					done, err := br.readN(1)

					switch {
					case err != nil:
						return mkError(errDecrunch, err)
					case done != 0:
						continue SubBlockScan // end of subblock, continue to next subblock
					}

					sym = 0xFF // unpacked value is 0xFF for this special case
				}
			}

			sym = int(transTable[sym])

			if delta {
				dacc += sym // update delta accumulator
				sym = dacc
				// sym += dacc // apply delta if needed
				// dacc = sym  // update delta accumulator
			}

			dst[i] = byte(sym & 0xFF)
			i++
		}
	}

	return nil
}

// decrunch16 decompresses 16-bit compressed data into the provided buffer.
func (r *Unpacker) decrunch16(in []byte, subBlocks []subBlock, bitLen int, delta bool, abs16 bool) error {
	out := r.x

	bit := newBitReader(in)

	dacc := 0 // delta accumulator is shared across subblocks

SubBlockScan:
	for _, sb := range subBlocks {
		if !sb.fits(out) {
			return mkError(errDecrunch, "subblock out of bounds")
		}

		lo, hi := sb.Pos, sb.Pos+sb.Size
		dst := out[lo:hi:hi]

		if len(dst)&1 != 0 {
			return mkError(errDecrunch, "odd size buffer for 16-bit data")
		}

		i := 0 // index in the destination buffer
		for i < len(dst) {
			// Read a symbol from the bitstream
			b, err := bit.readN(bitLen + 1)

			if err != nil {
				return mkError(errDecrunch, err)
			}

			sym := b

			if b >= command16[bitLen] { // command detected
				nfetch := fetch16[bitLen]
				nxtBitLen, err := bit.readN(nfetch) // read additional bits for the command

				if err != nil {
					return mkError(errDecrunch, err)
				}

				nxtBitLen += (b - command16[bitLen]) << nfetch // calculate new bit length

				if nxtBitLen != bitLen { // bit length change
					bitLen = nxtBitLen & 0x0F // update bit length for next iteration
					continue                  // skip to next symbol
				}

				b, err = bit.readN(4) // read 4 bits for the next command

				if err != nil {
					return mkError(errDecrunch, err)
				}

				sym = 0xFFF0 | b // unpacked value is 0xFFF0 + b for this command

				if b == 0x0F { // 0x0F indicates a special case
					done, err := bit.readN(1)

					switch {
					case done != 0:
						continue SubBlockScan // end of subblock, continue to next subblock
					case err != nil:
						return mkError(errDecrunch, err)
					}

					sym = 0xFFFF // unpacked value is 0xFFFF for this special case
				}
			}

			// Zizag mapping to signed delta
			if sym&1 != 0 {
				sym = -((sym + 1) >> 1)
			} else {
				sym = sym >> 1
			}

			switch {
			case delta:
				dacc += sym // update delta accumulator
				sym = dacc

				// sym += dacc // apply delta if needed
				// dacc = sym  // update delta accumulator
			case !abs16:
				sym ^= 0x8000 // apply absolute value transformation if needed
			}

			ble.PutUint16(dst[i:], uint16(sym))
			i += 2
		}
	}

	return nil
}

var ble = binary.LittleEndian // convenience alias for little-endian binary encoding

type fileHeader struct {
	Sig    [8]byte // "ziRCONia"
	HdrLen uint16
}

type tableHeader struct {
	Version uint16
	BlkCnt  uint16
	Size    uint32
	BlkOff  uint32

	CompGlob, CompFmt uint8 // unused but reserved fields
}

type block struct {
	FullSize    uint32
	CompSize    uint32
	XorCheck    uint32
	SubBlockCnt uint16
	Flags       uint16
	TTEntries   uint16
	BitLen      uint16
}

func (b block) isCompressed() bool {
	return b.Flags&Compressed != 0
}

func (b block) isBit16() bool {
	return b.Flags&Bit16 != 0
}

func (b block) isBit8() bool {
	return !b.isBit16()
}

func (b block) isABS16() bool {
	return b.Flags&ABS16 != 0
}

func (b block) isDelta() bool {
	return b.Flags&Delta != 0
}

type subBlock struct {
	Pos  uint32
	Size uint32
}

// fits checks if the subblock fits within the provided buffer.
func (rb subBlock) fits(buf []byte) bool {
	hi := int(rb.Pos + rb.Size)

	return 0 <= hi && hi <= len(buf)
}

var (
	command8 = [8]int{0x01, 0x03, 0x07, 0x0F, 0x1E, 0x3C, 0x78, 0xF8} // zero padded

	fetch8 = [8]int{0x03, 0x03, 0x03, 0x03, 0x02, 0x01} // zero padded

	command16 = [16]int{
		0x0001, 0x0003, 0x0007, 0x000F, 0x001E, 0x003C, 0x0078, 0x00F0,
		0x01F0, 0x03F0, 0x07F0, 0x0FF0, 0x1FF0, 0x3FF0, 0x7FF0, 0xFFF0,
	}

	fetch16 = [16]int{0x0004, 0x0004, 0x0004, 0x0004, 0x0003, 0x0002, 0x0001} // zero padded
)

const (
	Compressed = 0x0001
	Delta      = 0x0002
	Bit16      = 0x0004
	Stereo     = 0x0100
	ABS16      = 0x0200
	Endian     = 0x0400
)

func bread(r io.Reader, data any) error {
	return binary.Read(r, ble, data)
}

const transTableLen = 256

const MB = 1024 * 1024
