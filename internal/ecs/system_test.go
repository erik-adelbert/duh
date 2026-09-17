// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"slices"
	"testing"
)

type testSystem struct {
	name     string
	priority int
	once     bool
	updated  *bool
}

func (ts *testSystem) Name() string  { return ts.name }
func (ts *testSystem) Priority() int { return ts.priority }
func (ts *testSystem) Once() bool    { return ts.once }
func (ts *testSystem) Update(w *World) {
	if ts.updated != nil {
		*ts.updated = true
	}
}

type testInitSystem struct {
	testSystem
	initialized *bool
}

func (tis *testInitSystem) Initialize(w *World) {
	if tis.initialized != nil {
		*tis.initialized = true
	}
}

func TestRegisterSystem_OrderAndDuplicate(t *testing.T) {
	w := &World{}

	s1 := &testSystem{name: "A", priority: SystemPriorityNormal}
	s2 := &testSystem{name: "B", priority: SystemPriorityLow}
	s3 := &testSystem{name: "C", priority: SystemPriorityHigh}
	s4 := &testSystem{name: "A", priority: SystemPriorityNormal} // duplicate name

	RegisterSystem(w, s1)
	RegisterSystem(w, s2)
	RegisterSystem(w, s3)
	RegisterSystem(w, s4) // Should overwrite s1

	if len(w.systems) != 3 {
		t.Fatalf("expected 3 systems, got %d", len(w.systems))
	}

	// Should be ordered: C (high), A (normal), B (low)
	if w.systems[0].Name() != "C" || w.systems[1].Name() != "A" || w.systems[2].Name() != "B" {
		t.Errorf("unexpected system order: got %v", []string{w.systems[0].Name(), w.systems[1].Name(), w.systems[2].Name()})
	}
}

func TestRegisterSystem_WithInit(t *testing.T) {
	w := &World{}

	initCalled := false

	s := &testInitSystem{
		testSystem:  testSystem{name: "InitSys", priority: SystemPriorityNormal},
		initialized: &initCalled,
	}

	RegisterSystem(w, s)

	if !initCalled {
		t.Error("expected Initialize to be called for WithInit system")
	}
}

func TestUnregisterSystem(t *testing.T) {
	w := &World{}

	s1 := &testSystem{name: "A", priority: 0}
	s2 := &testSystem{name: "B", priority: 0}

	RegisterSystem(w, s1)
	RegisterSystem(w, s2)

	UnregisterSystem(w, "A")

	if len(w.systems) != 1 || w.systems[0].Name() != "B" {
		t.Errorf("expected only system B to remain, got %v", slices.Clone(w.systems))
	}

	UnregisterSystem(w, "B")

	if len(w.systems) != 0 {
		t.Errorf("expected no systems, got %v", slices.Clone(w.systems))
	}
}

type testCloseSystem struct {
	testSystem
	closed *bool
}

func (tcs *testCloseSystem) Close() {
	if tcs.closed != nil {
		*tcs.closed = true
	}
}

func TestUnregisterWithCloser(t *testing.T) {
	w := &World{}

	closed := false

	s := &testCloseSystem{
		testSystem: testSystem{name: "CloseSys", priority: 0},
		closed:     &closed,
	}

	RegisterSystem(w, s)

	UnregisterSystem(w, "CloseSys")

	if !closed {
		t.Error("expected Close to be called for WithCloser system")
	}
}

func TestUnregisterInexistentSystem(t *testing.T) {
	w := &World{}

	s1 := &testSystem{name: "A", priority: 0}

	RegisterSystem(w, s1)

	UnregisterSystem(w, "NonExistent")

	if len(w.systems) != 1 || w.systems[0].Name() != "A" {
		t.Errorf("expected system A to remain unchanged, got %v", slices.Clone(w.systems))
	}
}
