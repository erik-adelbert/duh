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
