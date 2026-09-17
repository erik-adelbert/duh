// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"fmt"
	"slices"
	"testing"

	"github.com/erik-adelbert/duh/internal/id"
)

var world *World

type foo[T any] struct {
	Value T
}

type comp1 int
type comp2 string
type comp3 float64
type comp4 []byte
type comp5 struct {
	A int
	B string
	C float64
}

type comp6 struct {
	X []byte
	Y foo[int]
}

type ent1 struct{}

type ent2 struct{}

type ent3 struct{}

type ent4 struct{}

type ent5 struct{}

// fake usage
var (
	_ ent1
	_ ent2
	_ ent3
	_ ent4
	_ ent5
)

const auto = AutoName

func init() {
	w := NewWorld()

	e1 := NewEntity(w)
	SetComponent[comp1](w, auto, e1, 10)
	SetComponent[comp2](w, auto, e1, "hello")

	e2 := NewEntity(w)
	SetComponent[comp2](w, auto, e2, "world")
	SetComponent[comp3](w, auto, e2, 3.14)

	e3 := NewEntity(w)
	SetComponent[comp1](w, auto, e3, 20)
	SetComponent[comp3](w, auto, e3, 2.718)

	e4 := NewEntity(w)
	SetComponent[comp4](w, "comp4", e4, []byte{1, 2, 3})
	SetComponent(w, "comp5", e4, foo[int]{Value: 42})

	e5 := NewEntity(w)
	SetComponent[comp1](w, auto, e5, 30)
	SetComponent[comp4](w, "comp4", e5, []byte{4, 5, 6})
	SetComponent(w, "comp5", e5, foo[int]{Value: 84})

	for i := range 100_000 {
		e100K0 := NewEntity(w)
		e100K1 := NewEntity(w)

		SetComponent(w, auto, e100K0, comp5{A: i})
		SetComponent(w, auto, e100K0, comp6{
			X: []byte("data"),
			Y: foo[int]{Value: i},
		})

		SetComponent(w, auto, e100K1, comp5{A: i * 2})
		SetComponent(w, auto, e100K1, comp6{
			X: []byte("data"),
			Y: foo[int]{Value: i * 2},
		})
	}

	world = w
}

func TestAllEntitiesWith(t *testing.T) { // Test entities with comp1
	c1 := NameFor[comp1]()
	c2 := NameFor[comp2]()
	c3 := NameFor[comp3]()

	got := slices.Collect(AllEntitiesWith(world, c1))

	if len(got) != 3 {
		t.Errorf("expected 3 entities with comp1, got %d", len(got))
	}

	// Test entities with comp2
	got = slices.Collect(AllEntitiesWith(world, c2))

	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp2, got %d", len(got))
	}

	// Test entities with comp3
	got = slices.Collect(AllEntitiesWith(world, c3))

	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp3, got %d", len(got))
	}

	// Test entities with comp1 and comp2
	got = slices.Collect(AllEntitiesWith(world, c1, c2))

	if len(got) != 1 {
		t.Errorf("expected 1 entity with comp1 and comp2, got %d", len(got))
	}

	// Test entities with comp1 and comp3
	got = slices.Collect(AllEntitiesWith(world, c1, c3))

	if len(got) != 1 {
		t.Errorf("expected 1 entity with comp1 and comp3, got %d", len(got))
	}

	// Test entities with comp2 and comp3
	got = slices.Collect(AllEntitiesWith(world, c2, c3))

	if len(got) != 1 {
		t.Errorf("expected 1 entity with comp2 and comp3, got %d", len(got))
	}

	// Test entities with all three components (should be none)
	got = slices.Collect(AllEntitiesWith(world, c1, c2, c3))

	if len(got) != 0 {
		t.Errorf("expected no entity with comp1, comp2, comp3, got %d", len(got))
	}

	got = slices.Collect(AllEntitiesWith(world, "comp4", "comp5"))

	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp4 and comp5, got %d", len(got))
	}

	got = slices.Collect(AllEntitiesWith(world, c1, "comp4", "comp5"))

	if len(got) != 1 {
		t.Errorf("expected 1 entity with comp1, comp4 and comp5, got %d", len(got))
	}
}

func TestAllEntitiesWithAny(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWithAny(world, NameFor[comp1]()) {
		got = append(got, eid)
	}

	if len(got) != 3 {
		t.Errorf("expected 3 entities with comp1, got %d", len(got))
	}

	got = got[:0]
	for eid := range AllEntitiesWithAny(world, NameFor[comp2]()) {
		got = append(got, eid)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp2, got %d", len(got))
	}

	got = got[:0]
	for eid := range AllEntitiesWithAny(world, NameFor[comp3]()) {
		got = append(got, eid)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp3, got %d", len(got))
	}

	got = got[:0]
	for eid := range AllEntitiesWithAny(world, NameFor[comp1](), NameFor[comp2]()) {
		got = append(got, eid)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 entities with comp1 or comp2, got %d", len(got))
	}

	got = got[:0]
	for eid := range AllEntitiesWithAny(world, NameFor[comp1](), NameFor[comp2](), NameFor[comp3]()) {
		got = append(got, eid)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 entities with any of comp1, comp2, comp3, got %d", len(got))
	}
}

func TestFirstEntityWith(t *testing.T) {
	eid := First(AllEntitiesWith(world, NameFor[comp1](), NameFor[comp2]()))
	if eid == id.Null() {
		t.Errorf("expected a valid entity with comp1 and comp2")
	}

	eid2 := First(AllEntitiesWith(world, NameFor[comp1](), NameFor[comp2](), NameFor[comp3]()))

	if eid2 != id.Null() {
		t.Errorf("expected no entity with comp1, comp2, comp3")
	}
}

func TestFirstEntityWithAny(t *testing.T) {
	eid := First(AllEntitiesWithAny(world, NameFor[comp1](), NameFor[comp2]()))

	if eid == id.Null() {
		t.Errorf("expected a valid entity with comp1 or comp2")
	}

	eid2 := First(AllEntitiesWithAny(world, "nonexistent"))

	if eid2 != id.Null() {
		t.Errorf("expected no entity with nonexistent component")
	}
}

func TestAllEntitiesWith_Empty(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWith(world) {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity when no components specified, got %d", len(got))
	}
}

func TestAllEntitiesWithAny_Empty(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWithAny(world) {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity when no components specified, got %d", len(got))
	}
}

func TestAllEntitiesWith_NonexistentComponent(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWith(world, "nonexistent") {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity with nonexistent component, got %d", len(got))
	}
}

func TestAllEntitiesWithAny_NonexistentComponent(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWithAny(world, "nonexistent") {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity with nonexistent component, got %d", len(got))
	}
}

func TestAllEntitiesWithAny_MixedComponents(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWithAny(world, NameFor[comp1](), "nonexistent") {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity with comp1 or nonexistent, got %d", len(got))
	}
}

func TestAllEntitiesWith_MixedComponents(t *testing.T) {
	var got []id.ID

	for eid := range AllEntitiesWith(world, NameFor[comp1](), "nonexistent") {
		got = append(got, eid)
	}

	if len(got) != 0 {
		t.Errorf("expected no entity with comp1 and nonexistent, got %d", len(got))
	}
}

func TestAllEntitiesWith_OrderIndependence(t *testing.T) {
	ids1 := []id.ID{}
	ids2 := []id.ID{}

	c1, c2 := NameFor[comp1](), NameFor[comp2]()

	for eid := range AllEntitiesWith(world, c1, c2) {
		ids1 = append(ids1, eid)
	}

	for eid := range AllEntitiesWith(world, c2, c1) {
		ids2 = append(ids2, eid)
	}

	if !slices.Equal(ids1, ids2) {
		t.Errorf("expected order independence, got %v and %v", ids1, ids2)
	}
}

func TestAllEntitiesMatching(t *testing.T) {
	c1 := NameFor[comp1]()

	p1 := NewPredicate(c1, func(c comp1) bool {
		return c > 15
	})

	got := slices.Collect(AllEntitiesMatching(world, p1))

	if len(got) != 2 {
		t.Errorf("expected 2 entities with comp1 > 15, got %d", len(got))
	}
}

func BenchmarkNameFor(b *testing.B) {
	for b.Loop() {
		NameFor[comp1]()
	}
}

func BenchmarkAllEntitiesWithSingleComponent(b *testing.B) {
	c1 := NameFor[comp1]()

	b.ResetTimer()

	for b.Loop() {
		for range AllEntitiesWith(world, c1) {
		}
	}
}

func BenchmarkFirstEntityWithSingleComponent(b *testing.B) {
	c1 := NameFor[comp1]()

	b.ResetTimer()

	for b.Loop() {
		First(AllEntitiesWith(world, c1))
	}
}

func BenchmarkAllEntitiesWithDualComponents(b *testing.B) {
	c1, c2 := NameFor[comp1](), NameFor[comp2]()

	for b.Loop() {
		for range AllEntitiesWith(world, c1, c2) {
		}
	}
}

func BenchmarkAllEntitiesWithTripleComponents(b *testing.B) {
	c1, c2, c3 := NameFor[comp1](), NameFor[comp2](), NameFor[comp3]()

	for b.Loop() {
		for range AllEntitiesWith(world, c1, c2, c3) {
		}
	}
}

func BenchmarkAllEntitiesWithAny(b *testing.B) {
	c1, c2 := NameFor[comp1](), NameFor[comp2]()

	for b.Loop() {
		for range AllEntitiesWithAny(world, c1, c2) {
		}
	}
}

func BenchmarkFirstEntityWith(b *testing.B) {
	c1, c2 := NameFor[comp1](), NameFor[comp2]()

	for b.Loop() {
		First(AllEntitiesWith(world, c1, c2))
	}
}

func BenchmarkFirstEntityWithAny(b *testing.B) {
	c1, c2 := NameFor[comp1](), NameFor[comp2]()

	for b.Loop() {
		First(AllEntitiesWithAny(world, c1, c2))
	}
}

func BenchmarkAllEntitiesMatching(b *testing.B) {
	c1 := NameFor[comp1]()

	p1 := NewPredicate(c1, func(c comp1) bool {
		return c > 15
	})

	for b.Loop() {
		for range AllEntitiesMatching(world, p1) {
		}
	}
}

func BenchmarkAllEntitiesWith100K(b *testing.B) {
	c6, c7 := NameFor[comp5](), NameFor[comp6]()

	for b.Loop() {
		for range AllEntitiesWith(world, c6, c7) {
		}
	}
}

func BenchmarkAllEntitiesWithAny100K(b *testing.B) {
	c6, c7 := NameFor[comp5](), NameFor[comp6]()

	for b.Loop() {
		for range AllEntitiesWithAny(world, c6, c7) {
		}
	}
}
func BenchmarkAllEntitiesWith_Matrix(b *testing.B) {
	componentCounts := []int{1, 2, 3, 5, 10, 50, 100, 1000, 10000}
	entityCounts := []int{1, 2, 3, 10, 100}

	for _, nComps := range componentCounts {
		for _, nEnts := range entityCounts {
			b.Run(
				// Name for sub-benchmark
				// e.g. "comps=10/ents=2"
				"comps="+itoa(nComps)+"/ents="+itoa(nEnts),
				func(b *testing.B) {
					w := NewWorld()

					compNames := make([]string, nComps)

					for i := range compNames {
						compNames[i] = "bench.comp1." + itoa(i)
					}
					// Create entities and assign all components
					for e := range nEnts {
						eid := NewEntity(w)

						for _, cname := range compNames {
							SetComponent(w, cname, eid, comp1(e))
						}
					}

					for b.Loop() {
						count := 0

						for range AllEntitiesWith(w, compNames...) {
							count++
						}

						if count != nEnts {
							b.Fatalf("expected %d entities, got %d", nEnts, count)
						}
					}
				},
			)
		}
	}
}

// Helper function to avoid strconv.Itoa import
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}
