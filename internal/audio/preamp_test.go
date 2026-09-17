// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/help"
)

func TestAttenuation(t *testing.T) {
	// Test cases: index, expected LUT index, expected value
	tests := []struct {
		index         int
		expectedIndex int
		expected      uint
	}{
		{-10, 0, PreAmpLUT[0]},  // Below lower bound, should clamp to 0
		{0, 0, PreAmpLUT[0]},    // Exact lower bound
		{1, 0, PreAmpLUT[0]},    // 1/2 = 0
		{2, 1, PreAmpLUT[1]},    // 2/2 = 1
		{7, 3, PreAmpLUT[3]},    // 7/2 = 3
		{8, 4, PreAmpLUT[4]},    // 8/2 = 4
		{15, 7, PreAmpLUT[7]},   // 15/2 = 7
		{16, 8, PreAmpLUT[8]},   // 16/2 = 8
		{23, 11, PreAmpLUT[11]}, // 23/2 = 11
		{30, 15, PreAmpLUT[15]}, // 30/2 = 15
		{31, 15, PreAmpLUT[15]}, // 31/2 = 15
		{40, 15, PreAmpLUT[15]}, // Above upper bound, should clamp to 31
	}

	for _, tc := range tests {
		got := Attenuation(tc.index)

		if got != tc.expected {
			t.Errorf("Attenuation(%d) = %d; want %d (LUT index %d)", tc.index, got, tc.expected, tc.expectedIndex)
		}
	}
}

// Optionally, test that util.Clamp is behaving as expected for completeness
func TestClamp(t *testing.T) {
	tests := []struct {
		val, min, max, want int
	}{
		{-5, 0, 31, 0},
		{0, 0, 31, 0},
		{15, 0, 31, 15},
		{31, 0, 31, 31},
		{40, 0, 31, 31},
	}

	for _, tc := range tests {
		got := help.Clamp(tc.val, tc.min, tc.max)

		if got != tc.want {
			t.Errorf("Clamp(%d, %d, %d) = %d; want %d", tc.val, tc.min, tc.max, got, tc.want)
		}
	}
}
