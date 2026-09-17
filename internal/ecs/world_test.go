// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/id"
)

func TestNewWorld_InitializesFields(t *testing.T) {
	w := NewWorld()

	if w == nil {
		t.Fatal("NewWorld returned nil")
	}

	if i, _ := w.nextID(); i != id.ID(1) {
		t.Errorf("expected nextID to be 1, got %v", i)
	}

	if len(w.entities) != 0 {
		t.Errorf("expected entities to be empty, got %v", w.entities)
	}

	if w.systems == nil {
		t.Error("expected systems to be initialized, got nil")
	}

	if len(w.systems) != 0 {
		t.Errorf("expected systems to be empty, got %v", w.systems)
	}

	if w.interner == nil {
		t.Error("expected interner to be initialized, got nil")
	}
}

// Mock system that implements Once and Update
type mockBaseSystem struct {
	updateCalled   *bool
	priorityCalled *bool
	onceCalled     *bool
}

func (m *mockBaseSystem) Name() string {
	return "mockSystem"
}

func (m *mockBaseSystem) Update(w *World) {
	if m.updateCalled != nil {
		*m.updateCalled = true
	}
}

func (m *mockBaseSystem) Priority() int {
	if m.priorityCalled != nil {
		*m.priorityCalled = true
	}

	return SystemPriorityNormal
}

type mockOnceSystem struct {
	mockBaseSystem
}

func (m *mockBaseSystem) Once() bool {
	if m.onceCalled != nil {
		*m.onceCalled = true
	}

	return true
}

type mockPersistentSystem struct {
	mockBaseSystem
}

func (m *mockPersistentSystem) Once() bool {
	if m.onceCalled != nil {
		*m.onceCalled = true
	}

	return false
}

func TestWorld_Update_RemovesOnceSystems(t *testing.T) {
	called := false

	w := NewWorld()

	sys := &mockOnceSystem{mockBaseSystem{updateCalled: &called}}
	w.systems = append(w.systems, sys)

	w.Update()

	if !called {
		t.Error("expected Update to call system's Update method")
	}

	if len(w.systems) != 0 {
		t.Errorf("expected systems to be removed after Update, got %d", len(w.systems))
	}
}

func TestWorld_Update_KeepsNonOnceSystems(t *testing.T) {
	updateCalled := false
	onceCalled := false

	w := NewWorld()

	sys := &mockPersistentSystem{
		mockBaseSystem{updateCalled: &updateCalled, onceCalled: &onceCalled},
	}
	w.systems = append(w.systems, sys)

	w.Update()

	if !updateCalled {
		t.Error("expected Update to call system's Update method")
	}

	if !onceCalled {
		t.Error("expected Once to be called during Update")
	}

	if len(w.systems) != 1 {
		t.Errorf("expected systems to remain after Update, got %d", len(w.systems))
	}
}

func TestWorld_cid_ReturnsCorrectCid(t *testing.T) {
	w := NewWorld()

	name := "TestComponent"

	cidVal := id.ID(42)

	w.interner.ids[name] = cidVal

	gotCid, ok := w.cid(name)
	if !ok {
		t.Errorf("expected cid to be found for %q", name)
	}

	if gotCid != cidVal {
		t.Errorf("expected cid %v, got %v", cidVal, gotCid)
	}
}

func TestWorld_cid_ReturnsFalseForUnknownName(t *testing.T) {
	w := NewWorld()

	gotCid, ok := w.cid("UnknownComponent")

	if ok {
		t.Errorf("expected cid lookup to fail for unknown name, got cid %v", gotCid)
	}
}
