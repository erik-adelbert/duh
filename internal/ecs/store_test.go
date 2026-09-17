// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"fmt"
	"testing"

	"slices"

	"github.com/erik-adelbert/duh/internal/id"
)

func TestStore_ids_Empty(t *testing.T) {
	s := newStore[int](0)

	got := slices.Collect(s.ids())

	if len(got) != 0 {
		t.Errorf("ids() on empty store: got %v, want []", got)
	}
}

func TestStore_ids_NonEmpty(t *testing.T) {
	s := newStore[string](0)

	ids := []id.ID{1, 2, 3}

	values := []string{"a", "b", "c"}

	for i, eid := range ids {
		s.set(eid, values[i])
	}

	got := slices.Collect(s.ids())

	slices.Sort(got)
	slices.Sort(ids)

	if !slices.Equal(got, ids) {
		t.Errorf("ids() = %v, want %v", got, ids)
	}
}

func TestStore_ids_AfterDelete(t *testing.T) {
	s := newStore[float64](0)

	ids := []id.ID{10, 20, 30}

	for _, eid := range ids {
		s.set(eid, float64(eid))
	}

	s.delete(20)

	want := []id.ID{10, 30}

	got := slices.Collect(s.ids())

	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("ids() after delete = %v, want %v", got, want)
	}
}

func BenchmarkStore_StoreVariousSizes(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, numIDs := range sizes {
		b.Run(fmt.Sprintf("N=%d", numIDs), func(b *testing.B) {
			s := newStore[int](numIDs)

			b.ResetTimer()

			i := 0
			for b.Loop() {
				s.set(id.ID(i), i)
				i++
			}
		})
	}
}

var sink1 int

func BenchmarkLoad_VariousSizes(b *testing.B) {
	sizes := []int{10, 100, 1000, 10000, 100000}

	for _, numIDs := range sizes {
		b.Run(fmt.Sprintf("N=%d", numIDs), func(b *testing.B) {
			s := newStore[int](numIDs)

			for i := range numIDs {
				s.set(id.ID(i), i)
			}

			b.ResetTimer()

			i := 0
			for b.Loop() {
				sink1, _ = s.get(id.ID(i))
				i++

				if i >= numIDs {
					i = 0
				}
			}

		})
	}
}
