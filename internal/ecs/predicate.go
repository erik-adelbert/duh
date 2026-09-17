// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

type Predicate[T any] struct {
	target string
	test   func(T) bool
}

func NewPredicate[T any](name string, test func(T) bool) Predicate[T] {
	if name == AutoName {
		name = NameFor[T]()
	}

	return Predicate[T]{
		target: name,
		test:   test,
	}
}

func (p Predicate[T]) Target() string {
	return p.target
}

func (p Predicate[T]) Test(value T) bool {
	return p.test(value)
}
