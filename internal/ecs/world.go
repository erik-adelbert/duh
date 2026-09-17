// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"iter"

	"github.com/erik-adelbert/duh/internal/id"
)

type World struct {
	nextID   func() (id.ID, bool)
	stopID   func()
	entities []id.ID
	components
	systems []System
}

func NewWorld() *World {
	nextID, stopID := id.NewIDPuller(1)

	return &World{
		nextID:     nextID,
		stopID:     stopID,
		entities:   []id.ID{},
		components: mkComponents(),
		systems:    []System{},
	}
}

func (w *World) Close() {
	w.stopID()
}

func (w *World) newID() id.ID {
	id, _ := w.nextID()
	return id
}

func (w *World) Update() {
	permanents := w.systems[:0] // Use the same storage

	// Update all systems and collect the ones that are permanent
	for _, s := range w.systems {
		s.Update(w)

		if !s.Once() {
			permanents = append(permanents, s)
		}
	}

	w.systems = permanents
}

func (w *World) cid(name string) (id.ID, bool) {
	cid, ok := w.interner.ids[name]

	return cid, ok
}

type componentStore interface {
	ids() iter.Seq[id.ID]
	has(id.ID) bool
	delete(id.ID)
	len() int
}

type WithInit interface {
	Initialize(*World)
}

type WithCloser interface {
	Close()
}
