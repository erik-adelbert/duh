// duh ADL midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package main

import "sync"

type Cache[K, T any] sync.Map

func (c *Cache[K, T]) Load(key K) (T, bool) {
	v, ok := (*sync.Map)(c).Load(key)

	if !ok {
		var zero T
		return zero, false
	}

	return v.(T), true
}

func (c *Cache[K, T]) Store(key K, value T) {
	(*sync.Map)(c).Store(key, value)
}

func (c *Cache[K, T]) Clear() {
	(*sync.Map)(c).Clear()
}

func (c *Cache[K, T]) Range(f func(key K, value T) bool) {
	(*sync.Map)(c).Range(func(k, v any) bool {
		return f(k.(K), v.(T))
	})
}
