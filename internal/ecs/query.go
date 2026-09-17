// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"iter"
	"slices"
	"sync"

	"github.com/erik-adelbert/duh/internal/id"
)

// AllEntitiesWith returns an iterator over all entity IDs that have all of the specified components.
func AllEntitiesWith(w *World, components ...string) iter.Seq[id.ID] {
	cids, free := getCIDs(w, components...)

	defer free()

	if len(cids) == 0 {
		return emptyIDSeq()
	}

	return func(yield func(id.ID) bool) {
		base := w.stores[cids[0]]
	EntityScan:
		for eid := range base.ids() {
			for _, cid := range cids[1:] {
				if !w.stores[cid].has(eid) {
					continue EntityScan
				}
			}

			if !yield(eid) {
				return
			}
		}
	}
}

func AllEntitiesWithAny(w *World, components ...string) iter.Seq[id.ID] {
	cids, free := getCIDs(w, components...)

	defer free()

	if len(cids) == 0 {
		return emptyIDSeq()
	}

	return func(yield func(id.ID) bool) {
		seen := make(map[id.ID]struct{}, 32)

		for _, cid := range cids {
			base := w.stores[cid]

			for eid := range base.ids() {
				if _, ok := seen[eid]; ok {
					continue
				}

				seen[eid] = struct{}{}

				if !yield(eid) {
					return
				}
			}
		}
	}
}

func AllEntitiesMatching[T any](w *World, p Predicate[T]) iter.Seq[id.ID] {
	return func(yield func(id.ID) bool) {
		for eid := range AllEntitiesWith(w, p.Target()) {
			var comp T

			if !GetComponent(w, p.Target(), eid, &comp) {
				continue
			}

			if p.Test(comp) && !yield(eid) {
				return
			}
		}
	}
}

// First returns the first entity ID from the given sequence, or id.Null() if the sequence is empty.
func First(seq iter.Seq[id.ID]) id.ID {
	for eid := range seq {
		return eid // Return the first entity ID
	}

	return id.Null()
}

// And returns an iterator over entity IDs that are present in all of the given sequences.
func And(seqs ...iter.Seq[id.ID]) iter.Seq[id.ID] {
	if len(seqs) == 0 {
		return emptyIDSeq()
	}

	seens := make([]map[id.ID]struct{}, len(seqs))
	frees := make([]func(), len(seqs))

	for i, seq := range seqs {
		seen, free := getIDSet()
		frees[i] = free

		for eid := range seq {
			seen[eid] = struct{}{}
		}

		seens[i] = seen
	}

	return func(yield func(id.ID) bool) {
		defer func() {
			for _, free := range frees {
				free()
			}
		}()

		base := seqs[0]
	EntityScan:
		for eid := range base {
			for _, seen := range seens[1:] {
				if _, ok := seen[eid]; !ok {
					continue EntityScan
				}
			}

			if !yield(eid) {
				return
			}
		}
	}
}

// Or returns an iterator over entity IDs that are present in any of the given sequences.
func Or(seqs ...iter.Seq[id.ID]) iter.Seq[id.ID] {
	if len(seqs) == 0 {
		return emptyIDSeq()
	}

	return func(yield func(id.ID) bool) {
		seen, free := getIDSet()

		defer free()

		for _, seq := range seqs {
			for eid := range seq {
				if _, ok := seen[eid]; !ok {
					seen[eid] = struct{}{}

					if !yield(eid) {
						return
					}
				}
			}
		}
	}
}

// Not returns an iterator over entity IDs from 'seq' that are not present in 'exclude'.
func Not(seq iter.Seq[id.ID], exclude iter.Seq[id.ID]) iter.Seq[id.ID] {
	excludeSet, free := getIDSet()

	for eid := range exclude {
		excludeSet[eid] = struct{}{}
	}

	return func(yield func(id.ID) bool) {
		defer free()

		for eid := range seq {
			if _, ok := excludeSet[eid]; ok {
				continue
			}

			if !yield(eid) {
				return
			}
		}
	}
}

const (
	cidsPoolInitialSize = 8
	cidsPoolMaxSize     = 1024
)

var cidsPool = sync.Pool{
	New: func() any {
		cdis := make([]id.ID, 0, cidsPoolInitialSize)
		return &cdis
	},
}

func getCIDs(w *World, components ...string) ([]id.ID, func()) {
	pids := cidsPool.Get().(*[]id.ID)
	cids := *pids
	cids = cids[:0]

	for _, name := range components {
		cid, ok := w.cid(name)

		if !ok {
			// No entities can have a missing component
			if len(cids) <= cidsPoolMaxSize {
				*pids = (*pids)[:0]
				cidsPool.Put(pids)
			}

			return nil, func() {}
		}

		cids = append(cids, cid)
	}

	// Pick smallest component set first
	slices.SortFunc(cids, func(a, b id.ID) int {
		return w.stores[a].len() - w.stores[b].len()
	})

	return cids, func() {
		if len(cids) <= cidsPoolMaxSize {
			*pids = (*pids)[:0]
			cidsPool.Put(pids)
		}
	}
}

const (
	idMapInitialSize = 32
	maxMapSize       = 1024
)

var idMapPool = sync.Pool{
	New: func() any {
		return make(map[id.ID]struct{}, idMapInitialSize)
	},
}

func getIDSet() (map[id.ID]struct{}, func()) {
	m := idMapPool.Get().(map[id.ID]struct{})

	clear(m)

	return m, func() {
		if len(m) > maxMapSize {
			return // Don't put back large maps to avoid memory bloat
		}

		idMapPool.Put(m)
	}
}

func emptyIDSeq() iter.Seq[id.ID] {
	return slices.Values([]id.ID{})
}
