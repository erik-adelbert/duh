// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"slices"

	"github.com/erik-adelbert/duh/internal/id"
)

func NewEntity(w *World) id.ID {
	eid := w.newID()
	w.entities = append(w.entities, eid)

	return eid
}

func RemoveEntity(w *World, entity id.ID) {
	w.entities = slices.DeleteFunc(w.entities, func(e id.ID) bool {
		return e == entity
	})

	// Remove all components associated with this entity
	for _, store := range w.stores {
		store.delete(entity)
	}
}
