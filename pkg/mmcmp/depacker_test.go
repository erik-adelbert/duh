// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
	"testing"
)

func compressedFixture(flags, bitLen uint16, table, bits []byte, size uint32) []byte {
	const blockOffset = 28
	data := make([]byte, blockOffset+20+8)
	copy(data, "ziRCONia")
	binary.LittleEndian.PutUint16(data[8:], 14)
	binary.LittleEndian.PutUint16(data[10:], 0x1300)
	binary.LittleEndian.PutUint16(data[12:], 1)
	binary.LittleEndian.PutUint32(data[14:], size)
	binary.LittleEndian.PutUint32(data[18:], 24)
	binary.LittleEndian.PutUint32(data[24:], blockOffset)
	binary.LittleEndian.PutUint32(data[blockOffset:], size)
	binary.LittleEndian.PutUint32(data[blockOffset+4:], uint32(len(table)+len(bits)))
	binary.LittleEndian.PutUint16(data[blockOffset+12:], 1)
	binary.LittleEndian.PutUint16(data[blockOffset+14:], flags)
	binary.LittleEndian.PutUint16(data[blockOffset+16:], uint16(len(table)))
	binary.LittleEndian.PutUint16(data[blockOffset+18:], bitLen)
	binary.LittleEndian.PutUint32(data[blockOffset+24:], size)
	data = append(data, table...)
	return append(data, bits...)
}

func TestCompressedBlockBoundaries(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{"8-bit translation table", compressedFixture(Compressed, 0, []byte{0x41, 0x42}, make([]byte, 4), 32), bytes.Repeat([]byte{0x41}, 32)},
		{"16-bit", compressedFixture(Compressed|Bit16, 0, nil, make([]byte, 2), 32), bytes.Repeat([]byte{0, 0x80}, 16)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Unpack(bytes.NewReader(tt.data), int64(len(tt.data)))
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(r)
			if err != nil || !bytes.Equal(got, tt.want) {
				t.Fatalf("got %x, error %v, want %x", got, err, tt.want)
			}
		})
	}
}

func TestCompressedBlockRejectsTruncation(t *testing.T) {
	b := compressedFixture(Compressed, 0, []byte{0x41, 0x42}, make([]byte, 4), 32)
	shortStream := bytes.Clone(b)
	binary.LittleEndian.PutUint32(shortStream[32:], 5)
	for _, tt := range []struct {
		name string
		data []byte
		want string
	}{
		{"missing byte", b[:len(b)-1], "compressed data out of bounds"},
		{"short declared stream", shortStream, "unexpected end of bitstream"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r, err := Unpack(bytes.NewReader(tt.data), int64(len(tt.data)))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadAll(r); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCompressedBlockInvalidBitLength(t *testing.T) {
	for _, tt := range []struct {
		flags, bits uint16
	}{
		{Compressed, 8}, {Compressed | Bit16, 16},
	} {
		b := compressedFixture(tt.flags, tt.bits, []byte{0}, make([]byte, 4), 32)
		r, err := Unpack(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadAll(r); err == nil || !strings.Contains(err.Error(), "invalid bit length") {
			t.Fatalf("flags %#x bits %d: error %v", tt.flags, tt.bits, err)
		}
	}
}
