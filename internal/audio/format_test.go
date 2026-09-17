// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"testing"
)

func TestAsPCM8(t *testing.T) {
	tests := []struct {
		input    int8
		expected PCM8
	}{
		{0, PCM8(0x80)},
		{127, PCM8(0xFF)},
		{-128, PCM8(0x80 - 128)},
		{-1, PCM8(0x7F)},
	}

	for _, tt := range tests {
		result := AsPCM8(tt.input)

		if result != tt.expected {
			t.Errorf("AsPCM8(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestPCM8_AsInt8(t *testing.T) {
	tests := []struct {
		input    PCM8
		expected int8
	}{
		{PCM8(0x80), 0},
		{PCM8(0xFF), 127},
		{PCM8(0x00), -128},
		{PCM8(0x7F), -1},
	}

	for _, tt := range tests {
		result := tt.input.AsInt8()

		if result != tt.expected {
			t.Errorf("PCM8(%d).AsInt8() = %d, want %d", tt.input, result, tt.expected)
		}
	}
}
