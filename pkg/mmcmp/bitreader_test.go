// Copyright (c) 2024 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

import (
	"testing"
)

func TestBitReaderRead(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		bits     int
		expected int
	}{
		{
			name:     "read 1 bit",
			data:     []byte{0x01},
			bits:     1,
			expected: 1,
		},
		{
			name:     "read 8 bits",
			data:     []byte{0xFF},
			bits:     8,
			expected: 0xFF,
		},
		{
			name:     "read 16 bits",
			data:     []byte{0xFF, 0xFF},
			bits:     16,
			expected: 0xFFFF,
		},
		{
			name:     "read partial byte",
			data:     []byte{0x0F},
			bits:     4,
			expected: 0xF,
		},
		{
			name:     "read across bytes",
			data:     []byte{0xFF, 0x00},
			bits:     12,
			expected: 0x0FF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			br := newBitReader(tt.data)
			result, _ := br.readN(tt.bits)
			if result != tt.expected {
				t.Errorf("Read(%d) = %#x, want %#x", tt.bits, result, tt.expected)
			}
		})
	}
}

func TestBitReaderAlign(t *testing.T) {
	data := []byte{0xFF, 0x00, 0xFF}
	br := newBitReader(data)

	if _, err := br.readN(3); err != nil {
		t.Fatalf("Read(3) error = %v", err)
	}

	result, _ := br.readN(8)
	expected := 0x1F
	if result != expected {
		t.Errorf("After reading 3 bits, Read(8) = %#x, want %#x", result, expected)
	}
}

func TestBitReaderBytesRead(t *testing.T) {
	data := []byte{0xFF, 0xFF, 0xFF}
	br := newBitReader(data)

	if n, err := br.readN(0); err != nil || n != 0 {
		t.Fatalf("Read(0) = (%d, %v), want (0, nil)", n, err)
	}
	if br.idx != 0 {
		t.Errorf("Initial bytes read = %d, want 0", br.idx)
	}

	if _, err := br.readN(8); err != nil {
		t.Fatalf("Read(8) error = %v", err)
	}
	if br.idx != 1 {
		t.Errorf("After reading 8 bits, bytes read = %d, want 1", br.idx)
	}

	if _, err := br.readN(8); err != nil {
		t.Fatalf("Read(8) error = %v", err)
	}
	if br.idx != 2 {
		t.Errorf("After reading 16 bits, bytes read = %d, want 2", br.idx)
	}
}

func TestBitReaderMultipleReads(t *testing.T) {
	data := []byte{0xAB, 0xCD, 0xEF}
	br := newBitReader(data)

	val1, err := br.readN(4)
	if err != nil {
		t.Fatalf("First Read(4) error = %v", err)
	}
	val2, err := br.readN(4)
	if err != nil {
		t.Fatalf("Second Read(4) error = %v", err)
	}
	val3, err := br.readN(8)
	if err != nil {
		t.Fatalf("Read(8) error = %v", err)
	}

	if val1 != 0xB {
		t.Errorf("First Read(4) = %#x, want 0xB", val1)
	}
	if val2 != 0xA {
		t.Errorf("Second Read(4) = %#x, want 0xA", val2)
	}
	if val3 != 0xCD {
		t.Errorf("Read(8) = %#x, want 0xCD", val3)
	}
}

func TestBitReaderReadBeyondData(t *testing.T) {
	data := []byte{0xFF}
	br := newBitReader(data)

	_, err := br.readN(8)
	if err != nil {
		t.Fatalf("Read(8) error = %v", err)
	}

	n, err := br.readN(1)
	if err != nil {
		t.Errorf("Read(1) error = %v", err)
	}

	if n != 0 {
		t.Errorf("Read(1) beyond data = %#x, want 0", n)
	}

	n, err = br.readN(63)
	if err != nil {
		t.Errorf("Read(63) error = %v", err)
	}
	if n != 0 {
		t.Errorf("Read(63) beyond data = %#x, want 0", n)
	}
}
