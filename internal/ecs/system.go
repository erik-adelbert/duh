// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"slices"
)

type System interface {
	Name() string
	Update(w *World)
	Priority() int
	Once() bool
}

type Priority = int

const (
	SystemPriorityHigh   Priority = -100
	SystemPriorityNormal Priority = 0
	SystemPriorityLow    Priority = 100
)

// RegisterSystem adds a system to the world, maintaining order by priority and name.
func RegisterSystem(w *World, s System) {
	// Find the correct index to insert the system
	i := slices.IndexFunc(w.systems, func(sys System) bool {
		switch {
		case sys.Priority() > s.Priority():
			return true
		case sys.Priority() == s.Priority():
			return sys.Name() >= s.Name()
		}

		return false
	})

	switch i {
	case -1:
		// System has the lowest priority/name
		w.systems = append(w.systems, s) // Add to end
	default:
		// Upsert at i
		if w.systems[i].Name() != s.Name() {
			w.systems = append(w.systems, nil)   // make room
			copy(w.systems[i+1:], w.systems[i:]) // shift right
		}

		w.systems[i] = s // Overwrite old or insert new
	}

	// Call initializer if applicable
	if initer, ok := s.(WithInit); ok {
		initer.Initialize(w)
	}
}

func UnregisterSystem(w *World, name string) {
	i := slices.IndexFunc(w.systems, func(s System) bool {
		return s.Name() == name
	})

	if i == -1 {
		return
	}

	// Call closer if applicable
	if closer, ok := w.systems[i].(WithCloser); ok {
		closer.Close()
	}

	w.systems = slices.Delete(w.systems, i, i+1)
}
