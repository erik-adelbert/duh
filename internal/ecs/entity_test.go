// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"iter"
	"testing"

	"github.com/erik-adelbert/duh/internal/id"
)

func TestNewEntity_AddsEntityToWorld(t *testing.T) {
	nextID, stopID := id.NewIDPuller(1)
	defer stopID()

	w := &World{
		entities: []id.ID{},
		nextID:   nextID,
		stopID:   stopID,
	}

	eid := NewEntity(w)

	if len(w.entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(w.entities))
	}

	if w.entities[0] != eid {
		t.Fatalf("expected entity id %v, got %v", eid, w.entities[0])
	}
}

func TestRemoveEntity_RemovesEntityFromWorld(t *testing.T) {
	w := &World{
		entities: []id.ID{1, 2, 3},
		components: components{
			stores: []componentStore{},
		},
	}
	RemoveEntity(w, 2)
	if len(w.entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(w.entities))
	}
	for _, e := range w.entities {
		if e == 2 {
			t.Fatalf("entity 2 was not removed")
		}
	}
}

func TestRemoveEntity_RemovesComponents(t *testing.T) {
	ms1 := &mockStore{}
	ms2 := &mockStore{}

	w := &World{
		entities: []id.ID{1, 2},
		components: components{
			stores: []componentStore{ms1, ms2},
		},
	}
	RemoveEntity(w, 1)

	if len(ms1.deleted) != 1 || ms1.deleted[0] != 1 {
		t.Errorf("expected ms1 to delete entity 1, got %v", ms1.deleted)
	}

	if len(ms2.deleted) != 1 || ms2.deleted[0] != 1 {
		t.Errorf("expected ms2 to delete entity 1, got %v", ms2.deleted)
	}
}

type mockStore struct {
	deleted []id.ID
}

func (m *mockStore) has(eid id.ID) bool {
	return false
}

func (m *mockStore) delete(eid id.ID) {
	m.deleted = append(m.deleted, eid)
}

func (m *mockStore) ids() iter.Seq[id.ID] {
	return func(yield func(id.ID) bool) {}
}

func (m *mockStore) len() int {
	return 0
}
