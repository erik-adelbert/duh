// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import (
	"reflect"

	"github.com/erik-adelbert/duh/internal/id"
)

const AutoName = ""

func NameFor[T any]() string {
	return reflect.TypeFor[T]().String()
}

func Register[T any](w *World, name string) id.ID {
	if name == AutoName {
		name = NameFor[T]()
	}

	cid := w.interner.index(name)

	for int(cid) >= len(w.stores) {
		w.stores = append(w.stores, nil)
		w.typed = append(w.typed, nil)
	}

	if w.stores[cid] != nil {
		return cid
	}

	s := newStore[T](0)
	w.stores[cid] = s
	w.typed[cid] = s

	return cid
}

func SetComponent[T any](w *World, name string, entity id.ID, comp T) {
	if name == AutoName {
		name = NameFor[T]()
	}

	cid, ok := w.cid(name)

	if !ok {
		cid = Register[T](w, name)
	}

	setComp(w, cid, entity, comp)
}

func GetComponent[T any](w *World, name string, eid id.ID, dst *T) (ok bool) {
	*dst, ok = ComponentAs[T](w, name, eid)
	return
}

func ComponentAs[T any](w *World, name string, entity id.ID) (T, bool) {
	if name == AutoName {
		name = NameFor[T]()
	}

	cid, ok := w.cid(name)

	if !ok {
		var zero T

		return zero, false
	}

	return getComp[T](w, cid, entity)
}

func DeleteComponent(w *World, name string, entity id.ID) {
	cid, ok := w.interner.ids[name]

	if !ok {
		return
	}

	w.stores[cid].delete(entity)
}

func setComp[T any](w *World, c, e id.ID, value T) {
	s := w.typed[c].(*store[T])

	s.set(e, value)
}

func getComp[T any](w *World, c, e id.ID) (T, bool) {
	s := w.typed[c].(*store[T])

	v, ok := s.get(e)

	return v, ok
}

type components struct {
	interner *indexer
	stores   []componentStore
	typed    []any
}

func mkComponents() components {
	return components{
		interner: newIndexer(),
		stores:   []componentStore{},
		typed:    []any{},
	}
}

type indexer struct {
	nextID id.ID
	ids    map[string]id.ID
	names  []string
}

func newIndexer() *indexer {
	return &indexer{
		ids: make(map[string]id.ID),
	}
}

func (in *indexer) index(name string) id.ID {
	var (
		i  id.ID
		ok bool
	)

	if i, ok = in.ids[name]; !ok {
		i = in.nextID
		in.nextID++

		in.ids[name] = i
		in.names = append(in.names, name)
	}

	return i
}
