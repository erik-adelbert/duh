// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package help

import (
	"cmp"
	"iter"
)

// Chunks divides a slice into chunks of the specified size.
// The last chunk may be smaller if the slice length is not a multiple of the chunk size.
// Chunks does not copy the underlying data, so modifying the original slice or the chunks
// will affect the data.
func Chunks[T any](buf []T, size int) iter.Seq[[]T] {
	if size <= 0 {
		return func(yield func([]T) bool) {}
	}

	return func(yield func([]T) bool) {
		for i := 0; i < len(buf); i += size {
			end := min(i+size, len(buf))

			if !yield(buf[i:end]) {
				return
			}
		}
	}
}

// Clamp clamps a value v to the range [a, b].
func Clamp[T cmp.Ordered](v, a, b T) T {
	v = max(v, a)
	v = min(v, b)

	return v
}
