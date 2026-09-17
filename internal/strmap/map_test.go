// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package strmap

import (
	"iter"
	"slices"
	"strconv"
	"testing"
)

func TestMapSetGet(t *testing.T) {
	m := NewMap[int]()

	// Test empty map
	if _, ok := m.Get("foo"); ok {
		t.Error("expected Get on empty map to return false")
	}

	// Insert and retrieve
	m.Set("foo", 42)

	v, ok := m.Get("foo")

	if !ok || v != 42 {
		t.Errorf("expected Get(foo) = 42, got %v, %v", v, ok)
	}

	// Update value
	m.Set("foo", 99)

	v, ok = m.Get("foo")

	if !ok || v != 99 {
		t.Errorf("expected Get(foo) = 99 after update, got %v, %v", v, ok)
	}

	// Insert another key
	m.Set("bar", 7)

	v, ok = m.Get("bar")

	if !ok || v != 7 {
		t.Errorf("expected Get(bar) = 7, got %v, %v", v, ok)
	}

	// Non-existent key
	if _, ok := m.Get("baz"); ok {
		t.Error("expected Get(baz) to return false")
	}
}

func TestMapDelete(t *testing.T) {
	m := NewMap[string]()

	m.Set("foo", "a")
	m.Set("bar", "b")
	m.Set("baz", "c")
	m.Delete("bar")

	if _, ok := m.Get("bar"); ok {
		t.Error("expected bar to be deleted")
	}

	if v, ok := m.Get("foo"); !ok || v != "a" {
		t.Error("foo should still exist")
	}

	if v, ok := m.Get("baz"); !ok || v != "c" {
		t.Error("baz should still exist")
	}

	// Delete non-existent key
	m.Delete("qux")
}

func TestMapKeysValuesItems(t *testing.T) {
	m := NewMap[int]()
	keys := []string{"a", "b", "abc", "bcd"}
	vals := []int{1, 2, 3, 4}

	for i, k := range keys {
		m.Set(k, vals[i])
	}

	gotKeys := slices.Collect(m.Keys())

	for _, k := range keys {
		found := slices.Contains(gotKeys, k)

		if !found {
			t.Errorf("missing key: %s", k)
		}
	}

	gotVals := slices.Collect(m.Values())

	for _, v := range vals {
		found := slices.Contains(gotVals, v)

		if !found {
			t.Errorf("missing value: %d", v)
		}
	}

	gotItems := m.Items()

	for _, v := range vals {
		found := slices.Contains(gotItems, v)

		if !found {
			t.Errorf("missing item: %d", v)
		}
	}
}

func TestMapAll(t *testing.T) {
	m := NewMap[string]()

	m.Set("foo", "bar")
	m.Set("baz", "qux")
	m.Set("f", "g")

	results := make(map[string]string)

	m.All()(func(k, v string) bool {
		results[k] = v

		return true
	})

	if len(results) != 3 {
		t.Errorf("expected 3 items, got %d", len(results))
	}

	if results["foo"] != "bar" || results["baz"] != "qux" || results["f"] != "g" {
		t.Errorf("unexpected All results: %v", results)
	}
}

func TestMapLenClear(t *testing.T) {
	m := NewMap[int]()

	if m.Len() != 0 {
		t.Errorf("expected Len=0, got %d", m.Len())
	}

	m.Set("a", 1)
	m.Set("b", 2)

	if m.Len() != 2 {
		t.Errorf("expected Len=2, got %d", m.Len())
	}

	m.Clear()

	if m.Len() != 0 {
		t.Errorf("expected Len=0 after Clear, got %d", m.Len())
	}

	if _, ok := m.Get("a"); ok {
		t.Error("expected Get(a) to be false after Clear")
	}
}

func TestMapPrefixOverlap(t *testing.T) {
	m := NewMap[int]()

	m.Set("foo", 1)
	m.Set("foobar", 2)
	m.Set("fo", 3)

	tests := []struct {
		key   string
		want  int
		found bool
	}{
		{"foo", 1, true},
		{"foobar", 2, true},
		{"fo", 3, true},
		{"f", 0, false},
		{"foob", 0, false},
	}

	for _, tt := range tests {
		got, ok := m.Get(tt.key)

		if ok != tt.found || (ok && got != tt.want) {
			t.Errorf("Get(%q) = %v, %v; want %v, %v", tt.key, got, ok, tt.want, tt.found)
		}
	}
}

func TestMapLarge(t *testing.T) {
	m := NewMap[int]()
	N := 10000

	for i := range N {
		m.Set("key"+strconv.Itoa(i), i)
	}

	for i := range N {
		v, ok := m.Get("key" + strconv.Itoa(i))

		if !ok || v != i {
			t.Errorf("Get(key%d) = %v, %v; want %d, true", i, v, ok, i)
		}
	}

	if m.Len() != N {
		t.Errorf("expected Len=%d, got %d", N, m.Len())
	}
}

func BenchmarkMapNew(b *testing.B) {
	for b.Loop() {
		_ = NewMap[int]()
	}
}

func BenchmarkStdSet(b *testing.B) {
	m := make(map[string]int)

	for i := range b.N {
		// m["key"+strconv.Itoa(i)] = i
		m["Modules.Metadata"] = i
	}
}

var sink int

func BenchmarkStdGet(b *testing.B) {
	N := 1000
	m := make(map[string]int)

	nums := make([]string, N)
	for i := range nums {
		nums[i] = "key" + strconv.Itoa(i)
	}

	for i, k := range nums {
		m[k] = i
	}

	b.ResetTimer()

	for i := range b.N {
		sink, _ = m[nums[i%N]]
	}
}

func BenchmarkMapSet(b *testing.B) {
	m := NewMap[int]()

	for i := range b.N {
		// m.Set("key"+strconv.Itoa(i), i)
		m.Set("Modules.Metadata", i)
	}
}

func BenchmarkMapGet(b *testing.B) {
	N := 1000
	m := NewMap[int]()

	nums := make([]string, N)
	for i := range nums {
		nums[i] = "key" + strconv.Itoa(i)
	}

	for i, s := range nums {
		m.Set(s, i)
	}

	b.ResetTimer()

	for i := range b.N {
		sink, _ = m.Get(nums[i%N])
	}
}

func BenchmarkWorstMapDelete(b *testing.B) {
	N := 1000
	m := NewMap[int]()

	nums := make([]string, N+1000)
	for i := range nums {
		nums[i] = "key" + strconv.Itoa(i)
	}

	for i, k := range nums {
		m.Set(k, i)
	}

	b.ResetTimer()

	for i := range b.N {
		m.Delete(nums[i%N])
	}
}

func BenchmarkMapKeys(b *testing.B) {
	N := 1000
	m := NewMap[int]()

	nums := make([]string, N)
	for i := range nums {
		nums[i] = "key" + strconv.Itoa(i)
	}

	for i, k := range nums {
		m.Set(k, i)
	}

	for b.Loop() {
		_ = m.Keys()
	}
}

func BenchmarkMapClear(b *testing.B) {
	N := 1000
	m := NewMap[int]()

	nums := make([]string, N)
	for i := range nums {
		nums[i] = "key" + strconv.Itoa(i)
	}

	for b.Loop() {
		m.Clear()
	}
}

func allVariants() iter.Seq[string] {
	return func(yield func(string) bool) {
		words := []string{"apple", "banana", "cherry", "date"}

		for _, w := range words {
			for _, v := range []string{w, w + "s", "my " + w} {
				if !yield(v) {
					return
				}
			}
		}
	}
}

func BenchmarkVariants(b *testing.B) {
	m := NewMap[struct{}]()

	for w := range allVariants() {
		m.Set(w, struct{}{})
	}

	for b.Loop() {
		for range m.All() {
		}
	}
}
