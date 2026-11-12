// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package offset

import (
	"errors"
	"math"
	"math/big"
	"testing"
)

func TestScaleBoundaries(t *testing.T) {
	values := []int64{math.MinInt64, math.MinInt64 + 1, -101, -1, 0, 1, 101, math.MaxInt64 - 1, math.MaxInt64}
	ratios := [][2]int64{{1, 1}, {2, 1}, {1, 2}, {3, 5}, {5, 3}, {math.MaxInt64, math.MaxInt64}, {math.MaxInt64, 2}}
	for _, v := range values {
		for _, ratio := range ratios {
			got, err := Scale(v, ratio[0], ratio[1])
			n := new(big.Int).Mul(big.NewInt(v), big.NewInt(ratio[1]))
			want := new(big.Int).Div(n, big.NewInt(ratio[0]))
			if !want.IsInt64() {
				if !errors.Is(err, ErrOverflow) {
					t.Errorf("Scale(%d,%d,%d) = %d, %v; want overflow", v, ratio[0], ratio[1], got, err)
				}
			} else if err != nil || got != want.Int64() {
				t.Errorf("Scale(%d,%d,%d) = %d, %v; want %s", v, ratio[0], ratio[1], got, err, want)
			}
		}
	}
	if _, err := Scale(1, 0, 1); !errors.Is(err, ErrInvalidScale) {
		t.Fatalf("invalid scale error = %v", err)
	}
}

func TestAddBoundaries(t *testing.T) {
	for _, tc := range []struct {
		a, b, want int64
		overflow   bool
	}{
		{math.MaxInt64, 1, 0, true}, {math.MinInt64, -1, 0, true},
		{math.MaxInt64, -1, math.MaxInt64 - 1, false},
		{math.MinInt64, 1, math.MinInt64 + 1, false},
	} {
		got, err := Add(tc.a, tc.b)
		if tc.overflow && !errors.Is(err, ErrOverflow) || !tc.overflow && (err != nil || got != tc.want) {
			t.Errorf("Add(%d,%d) = %d, %v", tc.a, tc.b, got, err)
		}
	}
}
