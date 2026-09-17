// duh xiter package
//
// # Based on unpublished work by rsc for the Go Team
//
// # Copyright (c) 2026, Erik Adelbert
//
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xiter

import (
	"iter"
	"reflect"
	"slices"
	"testing"
)

func TestStride(t *testing.T) {
	s := SliceStride([]int{10, 20, 30, 40, 50}, 2)

	got := collectSeq(s, 0)
	want := []int{10, 30, 50}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stride() = %v, want %v", got, want)
	}
}

func TestStride_ReturnsEmptyForNonPositiveStride(t *testing.T) {
	s := SliceStride([]int{10, 20, 30, 40, 50}, 0)

	got := collectSeq(s, 0)
	want := []int{}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stride() with stride=0 = %v, want %v", got, want)
	}

	s = SliceStride([]int{10, 20, 30, 40, 50}, -1)

	got = collectSeq(s, 0)
	want = []int{}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stride() with stride=-1 = %v, want %v", got, want)
	}
}

func TestStride_StopsWhenConsumerBreaks(t *testing.T) {
	s := SliceStride([]int{10, 20, 30, 40, 50}, 1)

	got := collectSeq(s, 2)
	want := []int{10, 20}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stride() with early stop = %v, want %v", got, want)
	}
}

func TestStride_Slice_StopsWhenConsumerBreaks(t *testing.T) {
	s := SliceStride([]int{10, 20, 30, 40, 50}, 1)
	got := collectSeq(s, 2)
	want := []int{10, 20}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Stride() with early stop = %v, want %v", got, want)
	}
}

func TestRepeat(t *testing.T) {
	got := collectSeq(Repeat("x", -1), 4)
	want := []string{"x", "x", "x", "x"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Repeat() = %v, want %v", got, want)
	}
}

func TestRepeat2(t *testing.T) {
	got := collectSeq2(Repeat2("a", 1, -1), 3)
	want := [][2]any{{"a", 1}, {"a", 1}, {"a", 1}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Repeat2() = %v, want %v", got, want)
	}
}

func TestZip2(t *testing.T) {
	a := slices.Values([]int{1, 2, 3})
	b := slices.Values([]string{"a", "b"})

	got := collectSeq2(Zip2(a, b), 0)
	want := [][2]any{{1, "a"}, {2, "b"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Zip2() = %v, want %v", got, want)
	}
}

func TestSliceZip2(t *testing.T) {
	got := collectSeq2(SliceZip2([]int{1, 2}, []string{"a", "b", "c"}), 0)
	want := [][2]any{{1, "a"}, {2, "b"}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SliceZip2() = %v, want %v", got, want)
	}
}

func BenchmarkZip2(b *testing.B) {
	left := make([]int, 1024)
	right := make([]int, 1024)

	for i := range left {
		left[i] = i
		right[i] = i * 2
	}

	a := slices.Values(left)
	bseq := slices.Values(right)
	total := 0

	b.ResetTimer()
	for b.Loop() {
		for x, y := range Zip2(a, bseq) {
			total += x + y
		}
	}

	if total == 0 {
		b.Fatal(total)
	}
}

func BenchmarkEqualFunc(b *testing.B) {
	left := make([]int, 1024)
	right := make([]int, 1024)

	for i := range left {
		left[i] = i
		right[i] = i
	}

	eq := func(x, y int) bool { return x == y }
	matched := false

	b.ResetTimer()
	for b.Loop() {
		matched = EqualFunc(slices.Values(left), slices.Values(right), eq)
	}

	if !matched {
		b.Fatal("EqualFunc returned false for equal sequences")
	}
}

func collectSeq[T any](seq iter.Seq[T], limit int) []T {
	values := make([]T, 0)

	for value := range seq {
		values = append(values, value)

		if limit > 0 && len(values) >= limit {
			break
		}
	}

	return values
}

func collectSeq2[A, B any](seq iter.Seq2[A, B], limit int) [][2]any {
	pairs := make([][2]any, 0)

	for a, b := range seq {
		pairs = append(pairs, [2]any{a, b})

		if limit > 0 && len(pairs) >= limit {
			break
		}
	}

	return pairs
}
