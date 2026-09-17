// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/id"
)

type testComponent struct {
	Value int
}

func newTestWorld() *World {
	return &World{
		components: mkComponents(),
	}
}

func TestRegister_NewComponentWithName(t *testing.T) {
	w := newTestWorld()
	name := "TestComponent"

	cid := Register[testComponent](w, name)

	if cid == 0 && len(w.stores) == 0 {
		t.Errorf("Register did not add a new store")
	}

	if w.stores[cid] == nil {
		t.Errorf("Store for cid %v is nil", cid)
	}

	if w.typed[cid] == nil {
		t.Errorf("Typed store for cid %v is nil", cid)
	}
}

func TestRegister_NewComponentWithAutoName(t *testing.T) {
	w := newTestWorld()

	cid := Register[testComponent](w, AutoName)

	expectedName := NameFor[testComponent]()

	gotCid, ok := w.interner.ids[expectedName]

	if !ok || gotCid != cid {
		t.Errorf("Component not registered with auto name, got cid %v for name %v", gotCid, expectedName)
	}
}

func TestRegister_ExistingComponentDoesNotOverwrite(t *testing.T) {
	w := newTestWorld()
	name := "TestComponent"

	cid1 := Register[testComponent](w, name)
	cid2 := Register[testComponent](w, name)

	if cid1 != cid2 {
		t.Errorf("Register returned different cids for the same name: %v vs %v", cid1, cid2)
	}

	if w.stores[cid1] == nil {
		t.Errorf("Store for cid %v is nil after second register", cid1)
	}
}

func TestComponentRoundtrip(t *testing.T) {
	w := newTestWorld()

	name := "TestComponent"
	entity := id.ID(1)
	comp := testComponent{Value: 42}

	SetComponent(w, name, entity, comp)

	got, ok := ComponentAs[testComponent](w, name, entity)
	if !ok {
		t.Errorf("ComponentAs did not find component for entity %v", entity)
	}

	if got != comp {
		t.Errorf("ComponentAs returned wrong value: got %+v, want %+v", got, comp)
	}
}

func TestComponentAs_NonExistentComponent(t *testing.T) {
	w := newTestWorld()

	name := "NonExistentComponent"
	entity := id.ID(2)

	got, ok := ComponentAs[testComponent](w, name, entity)
	if ok {
		t.Errorf("ComponentAs should not find non-existent component, got %+v", got)
	}
}

func TestComponentAs_AutoName(t *testing.T) {
	w := newTestWorld()

	entity := id.ID(3)
	comp := testComponent{Value: 99}

	SetComponent(w, AutoName, entity, comp)

	got, ok := ComponentAs[testComponent](w, AutoName, entity)

	if !ok {
		t.Errorf("ComponentAs did not find component with auto name for entity %v", entity)
	}

	if got != comp {
		t.Errorf("ComponentAs returned wrong value for auto name: got %+v, want %+v", got, comp)
	}
}

func TestRemoveComponent_RemovesComponent(t *testing.T) {
	w := newTestWorld()

	entity := id.ID(4)
	name := "TestComponent"
	comp := testComponent{Value: 123}

	SetComponent(w, name, entity, comp)
	DeleteComponent(w, name, entity)

	_, ok := ComponentAs[testComponent](w, name, entity)

	if ok {
		t.Errorf("Component was not removed for entity %v", entity)
	}
}

func TestRemoveComponent_NonExistentDoesNothing(t *testing.T) {
	w := newTestWorld()

	name := "NonExistentComponent"
	entity := id.ID(5)

	// Should not panic or error
	DeleteComponent(w, name, entity)
}

func BenchmarkComponentSet(b *testing.B) {
	w := newTestWorld()

	entity := id.ID(1)
	name := NameFor[testComponent]() // Benchmark with auto name
	comp := testComponent{Value: 42}

	for b.Loop() {
		SetComponent(w, name, entity, comp)
	}
}

func BenchmarkComponentGet(b *testing.B) {
	w := newTestWorld()

	entity := id.ID(1)
	name := NameFor[testComponent]() // Benchmark with auto name
	comp := testComponent{Value: 42}

	SetComponent(w, name, entity, comp)

	for b.Loop() {
		_, _ = ComponentAs[testComponent](w, name, entity)
	}
}

func BenchmarkComponentRoundtrip(b *testing.B) {
	w := newTestWorld()

	entity := id.ID(1)
	name := NameFor[testComponent]() // Benchmark with auto name
	comp := testComponent{Value: 66}

	for b.Loop() {
		SetComponent(w, name, entity, comp)
		_, _ = ComponentAs[testComponent](w, name, entity)
	}
}

func BenchmarkComponentRemove(b *testing.B) {
	w := newTestWorld()

	entity := id.ID(1)
	name := NameFor[testComponent]() // Benchmark with auto name
	comp := testComponent{Value: 77}

	for b.Loop() {
		SetComponent(w, name, entity, comp)
		DeleteComponent(w, name, entity)
	}
}
