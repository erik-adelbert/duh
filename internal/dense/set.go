// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dense

import (
	"iter"
	"slices"
)

const CapHint = 1_000

// Set is a generic dense set data structure that maps keys of type K to values of type V.
type Set[K comparable, V any] struct {
	dense  []V
	ids    []K
	sparse map[K]int
}

// NewSet creates a new empty Set.
func NewSet[K comparable, V any](capacity int) *Set[K, V] {
	capacity = max(capacity, CapHint) // min capacity is CapHint

	return &Set[K, V]{
		sparse: make(map[K]int, capacity),
		dense:  make([]V, 0, capacity),
		ids:    make([]K, 0, capacity),
	}
}

// Upsert adds or updates a key-value pair in the set.
func (ds *Set[K, V]) Upsert(id K, value V) {
	if _, ok := ds.sparse[id]; ok {
		// Update existing value
		ds.dense[ds.sparse[id]] = value

		return
	}

	// Insert new value
	ds.sparse[id] = len(ds.dense)
	ds.dense = append(ds.dense, value)
	ds.ids = append(ds.ids, id)
}

// Get retrieves the value associated with the given key.
func (ds *Set[K, V]) Get(id K) (V, bool) {
	if i, ok := ds.sparse[id]; ok {
		return ds.dense[i], true
	}

	var zero V

	return zero, false
}

// Delete removes the key-value pair associated with the given key from the set.
func (ds *Set[K, V]) Delete(id K) {
	i, ok := ds.sparse[id]

	if !ok {
		return
	}

	last := len(ds.dense) - 1
	ds.dense[i] = ds.dense[last]
	ds.ids[i] = ds.ids[last]
	ds.sparse[ds.ids[i]] = i
	ds.dense = ds.dense[:last]
	ds.ids = ds.ids[:last]

	delete(ds.sparse, id)
}

func (ds *Set[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i, v := range ds.dense {
			if !yield(ds.ids[i], v) {
				return
			}
		}
	}
}

func (ds *Set[K, V]) Keys() iter.Seq[K] {
	return slices.Values(ds.ids)
}

func (ds *Set[K, V]) Values() iter.Seq[V] {
	return slices.Values(ds.dense)
}

func (ds *Set[K, V]) Len() int {
	return len(ds.dense)
}
