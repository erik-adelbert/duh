// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"iter"

	"github.com/erik-adelbert/duh/internal/dense"
	"github.com/erik-adelbert/duh/internal/id"
)

type store[T any] struct {
	data *dense.Set[id.ID, T]
}

func newStore[T any](capacity int) *store[T] {
	return &store[T]{data: dense.NewSet[id.ID, T](capacity)}
}

func (s *store[T]) get(e id.ID) (T, bool) {
	return s.data.Get(e)
}

func (s *store[T]) set(e id.ID, v T) {
	s.data.Upsert(e, v)
}

func (s *store[T]) ids() iter.Seq[id.ID] {
	return s.data.Keys()
}

func (s *store[T]) has(e id.ID) bool {
	_, ok := s.data.Get(e)

	return ok
}

func (s *store[T]) delete(e id.ID) {
	s.data.Delete(e)
}

func (s *store[T]) len() int {
	return s.data.Len()
}
