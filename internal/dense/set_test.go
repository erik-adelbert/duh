// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dense

import (
	"maps"
	"slices"
	"testing"
)

const NumItems = 1_000

func checkNewSet[K comparable, V any](t *testing.T, set *Set[K, V]) {
	t.Helper()

	if set == nil {
		t.Fatal("NewSet returned nil")
	}

	if set.Len() != 0 {
		t.Errorf("expected Len() == 0, got %d", set.Len())
	}

	if set.sparse == nil {
		t.Error("sparse map is nil")
	}

	if len(set.dense) != 0 {
		t.Errorf("expected dense slice to be empty, got %d", len(set.dense))
	}

	if len(set.ids) != 0 {
		t.Errorf("expected ids slice to be empty, got %d", len(set.ids))
	}
}

func TestNewSet(t *testing.T) {
	type testCase[K comparable, V any] struct {
		name string
		new  func() *Set[K, V]
	}

	tests := []testCase[int, string]{
		{
			name: "IntString",

			new: func() *Set[int, string] { return NewSet[int, string](0) },
		},
	}
	tests2 := []testCase[string, int]{
		{
			name: "StringInt",

			new: func() *Set[string, int] { return NewSet[string, int](0) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checkNewSet(t, tc.new())
		})
	}

	for _, tc := range tests2 {
		t.Run(tc.name, func(t *testing.T) {
			checkNewSet(t, tc.new())
		})
	}
}

func TestUpsert_InsertAndUpdate(t *testing.T) {
	set := NewSet[int, string](0)

	// Insert new key-value pairs
	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")

	if set.Len() != 3 {
		t.Errorf("expected Len() == 3, got %d", set.Len())
	}

	// Check values
	val, ok := set.Get(1)

	if !ok || val != "one" {
		t.Errorf("expected Get(1) == 'one', got '%v', ok=%v", val, ok)
	}

	val, ok = set.Get(2)

	if !ok || val != "two" {
		t.Errorf("expected Get(2) == 'two', got '%v', ok=%v", val, ok)
	}

	val, ok = set.Get(3)

	if !ok || val != "three" {
		t.Errorf("expected Get(3) == 'three', got '%v', ok=%v", val, ok)
	}

	// Update existing key
	set.Upsert(2, "TWO")

	val, ok = set.Get(2)

	if !ok || val != "TWO" {
		t.Errorf("expected Get(2) == 'TWO' after update, got '%v', ok=%v", val, ok)
	}

	// Ensure length does not change after update
	if set.Len() != 3 {
		t.Errorf("expected Len() == 3 after update, got %d", set.Len())
	}
}

func TestUpsert_DifferentTypes(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 10)
	set.Upsert("b", 20)

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2, got %d", set.Len())
	}

	val, ok := set.Get("a")

	if !ok || val != 10 {
		t.Errorf("expected Get('a') == 10, got %d, ok=%v", val, ok)
	}

	val, ok = set.Get("b")

	if !ok || val != 20 {
		t.Errorf("expected Get('b') == 20, got %d, ok=%v", val, ok)
	}

	// Update value
	set.Upsert("a", 100)

	val, ok = set.Get("a")

	if !ok || val != 100 {
		t.Errorf("expected Get('a') == 100 after update, got %d, ok=%v", val, ok)
	}
}

func TestGet_ExistingAndNonExisting(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")

	// Test existing keys
	val, ok := set.Get(1)

	if !ok || val != "one" {
		t.Errorf("expected Get(1) == 'one', got '%v', ok=%v", val, ok)
	}

	val, ok = set.Get(2)

	if !ok || val != "two" {
		t.Errorf("expected Get(2) == 'two', got '%v', ok=%v", val, ok)
	}

	// Test non-existing key
	val, ok = set.Get(3)

	if ok {
		t.Errorf("expected Get(3) to return ok=false, got ok=%v, val='%v'", ok, val)
	}
}

func TestGet_ZeroValue(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 42)

	val, ok := set.Get("b")

	if ok {
		t.Errorf("expected Get('b') to return ok=false, got ok=%v, val=%d", ok, val)
	}

	if val != 0 {
		t.Errorf("expected zero value for int, got %d", val)
	}
}

func TestGet_AfterDelete(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Delete(1)

	val, ok := set.Get(1)

	if ok {
		t.Errorf("expected Get(1) after delete to return ok=false, got ok=%v, val='%v'", ok, val)
	}

	if val != "" {
		t.Errorf("expected zero value for string after delete, got '%v'", val)
	}
}

func TestDelete_ExistingKey(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")
	set.Delete(2)

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2 after delete, got %d", set.Len())
	}

	_, ok := set.Get(2)

	if ok {
		t.Errorf("expected Get(2) after delete to return ok=false")
	}

	// Ensure other keys are still present
	val, ok := set.Get(1)

	if !ok || val != "one" {
		t.Errorf("expected Get(1) == 'one', got '%v', ok=%v", val, ok)
	}

	val, ok = set.Get(3)

	if !ok || val != "three" {
		t.Errorf("expected Get(3) == 'three', got '%v', ok=%v", val, ok)
	}
}

func TestDelete_NonExistingKey(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 10)
	set.Upsert("b", 20)
	set.Delete("c") // "c" does not exist

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2 after deleting non-existing key, got %d", set.Len())
	}

	val, ok := set.Get("a")

	if !ok || val != 10 {
		t.Errorf("expected Get('a') == 10, got %d, ok=%v", val, ok)
	}

	val, ok = set.Get("b")

	if !ok || val != 20 {
		t.Errorf("expected Get('b') == 20, got %d, ok=%v", val, ok)
	}
}

func TestDelete_AllKeys(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Delete(1)
	set.Delete(2)

	if set.Len() != 0 {
		t.Errorf("expected Len() == 0 after deleting all keys, got %d", set.Len())
	}

	_, ok := set.Get(1)

	if ok {
		t.Errorf("expected Get(1) after delete to return ok=false")
	}

	_, ok = set.Get(2)

	if ok {
		t.Errorf("expected Get(2) after delete to return ok=false")
	}
}

func TestDelete_SingleElement(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("x", 99)
	set.Delete("x")

	if set.Len() != 0 {
		t.Errorf("expected Len() == 0 after deleting single element, got %d", set.Len())
	}

	val, ok := set.Get("x")

	if ok {
		t.Errorf("expected Get('x') after delete to return ok=false, got ok=%v, val=%d", ok, val)
	}

	if val != 0 {
		t.Errorf("expected zero value for int after delete, got %d", val)
	}
}

func TestAll_EmptySet(t *testing.T) {
	set := NewSet[int, string](0)

	var keys []int
	var values []string

	for k, v := range set.All() {
		keys = append(keys, k)
		values = append(values, v)
	}

	if len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}

	if len(values) != 0 {
		t.Errorf("expected no values, got %v", values)
	}
}

func TestAll_NonEmptySet(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 1)
	set.Upsert("b", 2)
	set.Upsert("c", 3)

	result := maps.Collect(set.All())

	expected := map[string]int{"a": 1, "b": 2, "c": 3}

	if len(result) != len(expected) {
		t.Errorf("expected %d elements, got %d", len(expected), len(result))
	}

	for k, v := range expected {
		if rv, ok := result[k]; !ok || rv != v {
			t.Errorf("expected key %q to have value %d, got %d", k, v, rv)
		}
	}
}

func TestAll_AfterDelete(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")
	set.Delete(2)

	result := maps.Collect(set.All())

	expected := map[int]string{1: "one", 3: "three"}

	if len(result) != len(expected) {
		t.Errorf("expected %d elements, got %d", len(expected), len(result))
	}

	for k, v := range expected {
		if rv, ok := result[k]; !ok || rv != v {
			t.Errorf("expected key %d to have value %q, got %q", k, v, rv)
		}
	}
}

func TestAll_EarlyStop(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")

	count := 0
	all := set.All()

	all(func(k int, v string) bool {
		count++

		return false // stop after first
	})

	if count != 1 {
		t.Errorf("expected early stop after 1 iteration, got %d", count)
	}
}

func TestKeys_EmptySet(t *testing.T) {
	set := NewSet[int, string](0)

	keys := slices.Collect(set.Keys())

	if len(keys) != 0 {
		t.Errorf("expected no keys, got %v", keys)
	}
}

func TestKeys_NonEmptySet(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 1)
	set.Upsert("b", 2)
	set.Upsert("c", 3)

	keys := slices.Collect(set.Keys())

	expected := map[string]bool{"a": true, "b": true, "c": true}

	if len(keys) != len(expected) {
		t.Errorf("expected %d keys, got %d", len(expected), len(keys))
	}

	for _, k := range keys {
		if !expected[k] {
			t.Errorf("unexpected key %q found", k)
		}
	}
}

func TestKeys_AfterDelete(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")
	set.Delete(2)

	keys := slices.Collect(set.Keys())

	expected := map[int]bool{1: true, 3: true}

	if len(keys) != len(expected) {
		t.Errorf("expected %d keys, got %d", len(expected), len(keys))
	}

	for _, k := range keys {
		if !expected[k] {
			t.Errorf("unexpected key %d found", k)
		}
	}
}

func TestKeys_EarlyStop(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")

	count := 0
	keys := set.Keys()

	keys(func(k int) bool {
		count++

		return false // stop after first
	})

	if count != 1 {
		t.Errorf("expected early stop after 1 iteration, got %d", count)
	}
}

func TestValues_EmptySet(t *testing.T) {
	set := NewSet[int, string](0)

	values := slices.Collect(set.Values())

	if len(values) != 0 {
		t.Errorf("expected no values, got %v", values)
	}
}

func TestValues_NonEmptySet(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 1)
	set.Upsert("b", 2)
	set.Upsert("c", 3)

	values := slices.Collect(set.Values())
	expected := map[int]bool{1: true, 2: true, 3: true}

	if len(values) != len(expected) {
		t.Errorf("expected %d values, got %d", len(expected), len(values))
	}

	for _, v := range values {
		if !expected[v] {
			t.Errorf("unexpected value %d found", v)
		}
	}
}

func TestValues_AfterDelete(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")
	set.Delete(2)

	values := slices.Collect(set.Values())

	expected := map[string]bool{"one": true, "three": true}

	if len(values) != len(expected) {
		t.Errorf("expected %d values, got %d", len(expected), len(values))
	}

	for _, v := range values {
		if !expected[v] {
			t.Errorf("unexpected value %q found", v)
		}
	}
}

func TestValues_EarlyStop(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(3, "three")

	count := 0
	values := set.Values()

	values(func(v string) bool {
		count++

		return false // stop after first
	})

	if count != 1 {
		t.Errorf("expected early stop after 1 iteration, got %d", count)
	}
}

func TestLen_EmptySet(t *testing.T) {
	set := NewSet[int, string](0)

	if set.Len() != 0 {
		t.Errorf("expected Len() == 0 for empty set, got %d", set.Len())
	}
}

func TestLen_AfterUpsert(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("a", 1)

	if set.Len() != 1 {
		t.Errorf("expected Len() == 1 after one upsert, got %d", set.Len())
	}

	set.Upsert("b", 2)

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2 after two upserts, got %d", set.Len())
	}
}

func TestLen_AfterUpdate(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Upsert(1, "ONE") // update existing key

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2 after update, got %d", set.Len())
	}
}

func TestLen_AfterDelete(t *testing.T) {
	set := NewSet[string, int](0)

	set.Upsert("x", 10)
	set.Upsert("y", 20)
	set.Delete("x")

	if set.Len() != 1 {
		t.Errorf("expected Len() == 1 after deleting one key, got %d", set.Len())
	}

	set.Delete("y")

	if set.Len() != 0 {
		t.Errorf("expected Len() == 0 after deleting all keys, got %d", set.Len())
	}
}

func TestLen_DeleteNonExistingKey(t *testing.T) {
	set := NewSet[int, string](0)

	set.Upsert(1, "one")
	set.Upsert(2, "two")
	set.Delete(3) // non-existing key

	if set.Len() != 2 {
		t.Errorf("expected Len() == 2 after deleting non-existing key, got %d", set.Len())
	}
}

// Benchmarks
func BenchmarkUpsert(b *testing.B) {
	set := NewSet[int, int](0)

	b.ResetTimer()

	i := 0
	for b.Loop() {
		set.Upsert(i, i)
		i++
	}
}

func BenchmarkStdUpsert(b *testing.B) {
	m := make(map[int]int)

	b.ResetTimer()

	i := 0
	for b.Loop() {
		m[i] = i
		i++
	}
}

func BenchmarkGet(b *testing.B) {
	set := NewSet[int, int](0)

	for i := range NumItems {
		set.Upsert(i, i)
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		_, _ = set.Get(k)
		k++

		if k >= NumItems {
			b.StopTimer()

			k = 0
			for i := range NumItems {
				set.Upsert(i, i)
			}

			b.StartTimer()
		}
	}
}

func BenchmarkStdGet(b *testing.B) {
	m := make(map[int]int)

	for i := range NumItems {
		m[i] = i
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		_, _ = m[k]
		k++
		if k >= NumItems {
			b.StopTimer()

			k = 0
			for i := range NumItems {
				m[i] = i
			}

			b.StartTimer()
		}
	}
}

func BenchmarkDelete(b *testing.B) {
	set := NewSet[int, int](0)

	for i := range NumItems {
		set.Upsert(i, i)
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		set.Delete(k)
		k++

		if k >= NumItems {
			b.StopTimer()

			k = 0
			for i := range NumItems {
				set.Upsert(i, i)
			}

			b.StartTimer()
		}
	}
}

func BenchmarkStdDelete(b *testing.B) {
	m := make(map[int]int)

	for i := range NumItems {
		m[i] = i
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		delete(m, k)
		k++

		if k >= NumItems {
			b.StopTimer()

			k = 0
			for i := range NumItems {
				m[i] = i
			}

			b.StartTimer()
		}
	}
}

func BenchmarkAll(b *testing.B) {
	set := NewSet[int, int](0)

	for i := range NumItems {
		set.Upsert(i, i)
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range set.All() {
			count++
		}
	}
}

func BenchmarkStdAll(b *testing.B) {
	m := make(map[int]int)

	for i := range NumItems {
		m[i] = i
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range m {
			count++
		}
	}
}

func BenchmarkKeys(b *testing.B) {
	set := NewSet[int, int](0)

	for i := range NumItems {
		set.Upsert(i, i)
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range set.Keys() {
			count++
		}
	}
}

func BenchmarkStdKeys(b *testing.B) {
	m := make(map[int]int)

	for i := range NumItems {
		m[i] = i
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range m {
			count++
		}
	}
}

func BenchmarkValues(b *testing.B) {
	set := NewSet[int, int](0)

	for i := range NumItems {
		set.Upsert(i, i)
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range set.Values() {
			count++
		}
	}
}

func BenchmarkStdValues(b *testing.B) {
	m := make(map[int]int)

	for i := range NumItems {
		m[i] = i
	}

	b.ResetTimer()

	for b.Loop() {
		count := 0

		for range m {
			count++
		}
	}
}
