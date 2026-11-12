// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package offset provides checked arithmetic for stream positions.
package offset

import (
	"errors"
	"math"
	"math/bits"
)

// ErrOverflow indicates that the result cannot be represented as an int64.
var ErrOverflow = errors.New("offset overflow")

// ErrInvalidScale indicates that a scale factor is not positive.
var ErrInvalidScale = errors.New("invalid offset scale")

// Scale computes floor(value * to / from), detecting unrepresentable results.
func Scale(value, from, to int64) (int64, error) {
	if from <= 0 || to <= 0 {
		return 0, ErrInvalidScale
	}
	neg := value < 0
	mag := uint64(value)

	if neg {
		mag = uint64(-(value + 1)) + 1
	}

	hi, lo := bits.Mul64(mag, uint64(to))
	if hi >= uint64(from) {
		return 0, ErrOverflow
	}

	q, rem := bits.Div64(hi, lo, uint64(from))
	if neg && rem != 0 {
		if q == math.MaxUint64 {
			return 0, ErrOverflow
		}
		q++
	}

	if neg {
		switch {
		case q > uint64(1)<<63:
			return 0, ErrOverflow
		case q == uint64(1)<<63:
			return math.MinInt64, nil
		}

		return -int64(q), nil
	}

	if q > math.MaxInt64 {
		return 0, ErrOverflow
	}

	return int64(q), nil
}

// Add adds two offsets, returning ErrOverflow if the result exceeds int64.
func Add(a, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		return 0, ErrOverflow
	}
	return a + b, nil
}
