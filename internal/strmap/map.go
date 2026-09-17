// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strmap

import (
	"iter"
	"slices"
	"unicode/utf8"
)

// Map is a radix trie mapping string -> T.
//
// Invariants:
//   - root.label == ""
//   - all non-root node.labels are non-empty
//   - children are sorted by label[0]
//   - value == noValue means no key terminates at this node
//   - nodes are arena-allocated and never freed
//   - items is append-only; deletes do not reclaim slots
type Map[T any] struct {
	root  int
	nodes []node
	items []T
}

// NewMap creates an empty Map.
func NewMap[T any]() *Map[T] {
	sm := &Map[T]{}
	sm.root = sm.allocNode("", noValue)

	return sm
}

func (sm *Map[T]) All() iter.Seq2[string, T] {
	return func(yield func(string, T) bool) {
		var recollect func(n int, prefix string) bool

		recollect = func(n int, prefix string) bool {
			node := sm.nodes[n]
			prefix += node.label

			if node.value != noValue {
				if !yield(prefix, sm.items[node.value]) {
					return false
				}
			}

			for _, child := range node.children {
				if !recollect(child, prefix) {
					return false
				}
			}

			return true
		}

		recollect(sm.root, "")
	}
}

func (sm *Map[T]) Items() []T {
	return sm.items
}

func (sm *Map[T]) Keys() iter.Seq[string] {
	return func(yield func(string) bool) {
		var recollect func(n int, prefix string)

		recollect = func(n int, prefix string) {
			node := sm.nodes[n]
			prefix += node.label

			if node.value != noValue {
				if !yield(prefix) {
					return
				}
			}

			for _, child := range node.children {
				recollect(child, prefix)
			}
		}

		recollect(sm.root, "")
	}
}

func (sm *Map[T]) Values() iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range sm.items {
			if !yield(v) {
				return
			}
		}
	}
}

func (sm *Map[T]) Clear() {
	sm.nodes = nil
	sm.items = nil
	sm.root = sm.allocNode("", noValue)
}

func (sm *Map[T]) Len() int {
	return len(sm.items)
}

// Get retrieves a value by key.
func (sm *Map[T]) Get(s string) (T, bool) {
	var zero T

	n := sm.root

	for {
		if len(s) == 0 {
			if sm.nodes[n].value == noValue {
				return zero, false
			}

			return sm.items[sm.nodes[n].value], true
		}

		_, child, ok := sm.findChild(n, s[0])

		if !ok {
			return zero, false
		}

		lbl := sm.nodes[child].label
		p := lcp(s, lbl)

		if p != len(lbl) {
			return zero, false
		}

		s = s[p:]
		n = child
	}
}

// Set inserts or updates a key.
func (sm *Map[T]) Set(s string, value T) {
	n := sm.root

	for {
		if len(s) == 0 {
			if sm.nodes[n].value == noValue {
				sm.nodes[n].value = len(sm.items)
				sm.items = append(sm.items, value)
			} else {
				sm.items[sm.nodes[n].value] = value
			}

			return
		}

		pos, child, ok := sm.findChild(n, s[0])

		if !ok {
			leaf := sm.allocNode(s, len(sm.items))
			sm.items = append(sm.items, value)
			kids := sm.nodes[n].children
			kids = append(kids, 0)

			copy(kids[pos+1:], kids[pos:])

			kids[pos] = leaf
			sm.nodes[n].children = kids

			return
		}

		lbl := sm.nodes[child].label
		p := lcp(s, lbl)

		if p == len(lbl) {
			s = s[p:]
			n = child

			continue
		}

		// Split existing node
		split := sm.allocNode(lbl[p:], sm.nodes[child].value)
		sm.nodes[split].children = sm.nodes[child].children
		sm.nodes[child].label = lbl[:p]
		sm.nodes[child].children = []int{split}
		sm.nodes[child].value = noValue

		if p == len(s) {
			sm.nodes[child].value = len(sm.items)
			sm.items = append(sm.items, value)
		} else {
			leaf := sm.allocNode(s[p:], len(sm.items))

			sm.items = append(sm.items, value)

			kids := sm.nodes[child].children

			i, _, _ := sm.findChild(child, sm.nodes[leaf].label[0])
			kids = append(kids, 0)

			copy(kids[i+1:], kids[i:])

			kids[i] = leaf
			sm.nodes[child].children = kids
		}

		return
	}
}

// Delete removes a key if present.
func (sm *Map[T]) Delete(s string) {
	var stack []int

	n := sm.root

	for len(s) > 0 {
		_, child, ok := sm.findChild(n, s[0])

		if !ok {
			return
		}

		lbl := sm.nodes[child].label
		p := lcp(s, lbl)

		if p != len(lbl) {
			return
		}

		stack = append(stack, n)
		s = s[p:]
		n = child
	}

	if sm.nodes[n].value == noValue {
		return
	}

	sm.nodes[n].value = noValue

	// Cleanup bottom-up
	for len(stack) > 0 {
		parent := stack[len(stack)-1] // pop
		stack = stack[:len(stack)-1]

		// Remove empty leaf
		if sm.nodes[n].value == noValue && len(sm.nodes[n].children) == 0 {
			kids := sm.nodes[parent].children

			for i, c := range kids {
				if c == n {
					sm.nodes[parent].children =
						append(kids[:i], kids[i+1:]...)

					break
				}
			}

			n = parent

			continue
		}

		// Merge single-child node (never merge root)
		if n != sm.root &&
			sm.nodes[n].value == noValue &&
			len(sm.nodes[n].children) == 1 {
			child := sm.nodes[n].children[0]
			sm.nodes[n].label += sm.nodes[child].label
			sm.nodes[n].value = sm.nodes[child].value
			sm.nodes[n].children = sm.nodes[child].children

			continue
		}

		break
	}
}

// allocNode allocates a node in the arena.
func (sm *Map[T]) allocNode(label string, value int) int {
	sm.nodes = append(sm.nodes, node{
		label: label,
		value: value,
	})

	return len(sm.nodes) - 1
}

// findChild finds a child whose label starts with byte b.
// Returns insertion position, child index, and whether it was found.
func (sm *Map[T]) findChild(n int, b byte) (pos int, child int, ok bool) {
	children := sm.nodes[n].children
	i, ok := slices.BinarySearchFunc(children, b, func(childID int, key byte) int {
		return int(sm.nodes[childID].label[0]) - int(key)
	})

	if ok {
		return i, children[i], true
	}

	return i, noValue, false
}

type node struct {
	label    string
	children []int // indices into nodes, sorted by label[0]
	value    int   // index into items, -1 = none
}

const noValue = -1

// lcp returns the length of the longest common prefix of a and b.
func lcp(a, b string) int {
	n := min(len(a), len(b))
	i := 0

	for i < n && a[i] == b[i] {
		i++
	}

	for i > 0 && i < n && !utf8.RuneStart(a[i]) {
		i--
	}

	return i
}
