// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package help

type Matrix[T any] struct {
	Data          []T
	Height, Width int
}

func NewMatrix[T any](height, width int) Matrix[T] {
	return Matrix[T]{
		Data:   make([]T, height*width),
		Height: height,
		Width:  width,
	}
}

func (m Matrix[T]) LinearIndex() func(r, c int) (i int) {
	return func(r, c int) int {
		return r*m.Width + c
	}
}

func (m Matrix[T]) Clone() (clone Matrix[T]) {
	newData := make([]T, len(m.Data))

	copy(newData, m.Data)

	clone = m
	clone.Data = newData

	return
}

func (m Matrix[T]) Rows() int {
	return m.Height
}

func (m Matrix[T]) Cols() int {
	return m.Width
}

func (m Matrix[T]) Get(r, c int) T {
	return m.Data[r*m.Width+c]
}

func (m Matrix[T]) Set(r, c int, val T) {
	m.Data[r*m.Width+c] = val
}
