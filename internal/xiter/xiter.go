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
	"cmp"
	"iter"
	"slices"

	"golang.org/x/exp/constraints"
)

func Any[T any](seq iter.Seq[T], pred func(T) bool) bool {
	for v := range seq {
		if pred(v) {
			return true
		}
	}

	return false
}

func Any2[K, V any](seq iter.Seq2[K, V], pred func(K, V) bool) bool {
	for k, v := range seq {
		if pred(k, v) {
			return true
		}
	}

	return false
}

// Concat concatenates multiple sequences into a single sequence.
func Concat[T any](seqs ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, seq := range seqs {
			for v := range seq {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Concat2 concatenates multiple sequences of key-value pairs into a single sequence.
func Concat2[K, V any](seqs ...iter.Seq2[K, V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, seq := range seqs {
			for k, v := range seq {
				if !yield(k, v) {
					return
				}
			}
		}
	}
}

// Equal reports whether two sequences are equal.
func Equal[T comparable](a, b iter.Seq[T]) bool {
	return EqualFunc(a, b, func(x, y T) bool { return x == y })
}

// EqualFunc reports whether two sequences are equal according to function eq.
func EqualFunc[T any](a, b iter.Seq[T], eq func(T, T) bool) bool {
	nextB, stopB := iter.Pull(b)

	defer stopB()

	for va := range a {
		vb, okb := nextB()

		if !okb || !eq(va, vb) {
			return false
		}
	}

	_, okb := nextB() // Check if b has more elements than a

	return !okb
}

// Equal2 reports whether two sequences of key-value pairs are equal.
func Equal2[K, V comparable](a, b iter.Seq2[K, V]) bool {
	return EqualFunc2(a, b, func(xk K, xv V, yk K, yv V) bool { return xk == yk && xv == yv })
}

// EqualFunc2 reports whether two sequences of key-value pairs are equal according to function eq.
func EqualFunc2[K, V any](a, b iter.Seq2[K, V], eq func(K, V, K, V) bool) bool {
	nextB, stopB := iter.Pull2(b)

	defer stopB()

	for ka, va := range a {
		kb, vb, okb := nextB()

		if !okb || !eq(ka, va, kb, vb) {
			return false
		}
	}

	_, _, okb := nextB() // Check if b has more elements than a

	return !okb
}

func Every[T any](seq iter.Seq[T], pred func(T) bool) bool {
	for v := range seq {
		if !pred(v) {
			return false
		}
	}

	return true
}

func Every2[K, V any](seq iter.Seq2[K, V], pred func(K, V) bool) bool {
	for k, v := range seq {
		if !pred(k, v) {
			return false
		}
	}

	return true
}

func EmptySeq[T any]() iter.Seq[T] {
	return func(yield func(T) bool) {}
}

func EmptySeq2[K, V any]() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {}
}

// Filter returns a sequence that yields only the elements of seq for which pred returns true.
func Filter[T any](seq iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if pred(v) {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// FilterZero returns a sequence that yields only the non-zero elements of seq.
func FilterZero[T comparable](seq iter.Seq[T]) iter.Seq[T] {
	var zero T

	return Filter(seq, func(v T) bool { return v != zero })
}

// Filter2 returns a sequence that yields only the key-value pairs of seq for which pred returns true.
func Filter2[K, V any](seq iter.Seq2[K, V], pred func(K, V) bool) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range seq {
			if pred(k, v) && !yield(k, v) {
				return
			}
		}
	}
}

func Length[T any](seq iter.Seq[T]) int {
	count := 0

	seq(func(_ T) bool {
		count++
		return true
	})

	return count
}

func Length2[K, V any](seq iter.Seq2[K, V]) int {
	count := 0

	seq(func(_ K, _ V) bool {
		count++
		return true
	})

	return count
}

// Map returns a sequence that applies function f to each element of seq.
func Map[T any, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}

// Map2 returns a sequence that applies function f to each key-value pair of seq.
func Map2[KIn, VIn, KOut, VOut any](seq iter.Seq2[KIn, VIn], f func(KIn, VIn) (KOut, VOut)) iter.Seq2[KOut, VOut] {
	return func(yield func(KOut, VOut) bool) {
		for k, v := range seq {
			if !yield(f(k, v)) {
				return
			}
		}
	}
}

// Merge merges two sorted sequences into a single sorted sequence.
func Merge[T cmp.Ordered](a, b iter.Seq[T]) iter.Seq[T] {
	return MergeFunc(a, b, cmp.Compare)
}

// MergeFunc merges two sorted sequences into a single sorted sequence according to function cmp.
func MergeFunc[T any](a, b iter.Seq[T], cmp func(T, T) int) iter.Seq[T] {
	return func(yield func(T) bool) {
		nextB, stopB := iter.Pull(b)

		defer stopB()

		vb, okb := nextB()

		for va := range a {
			for okb && cmp(va, vb) > 0 {
				if !yield(vb) {
					return
				}

				vb, okb = nextB()
			}

			if !yield(va) {
				return
			}
		}

		for okb {
			if !yield(vb) {
				return
			}

			vb, okb = nextB()
		}
	}
}

// Merge2 merges two sorted sequences of key-value pairs into a single sorted sequence.
func Merge2[K cmp.Ordered, V any](a, b iter.Seq2[K, V]) iter.Seq2[K, V] {
	return MergeFunc2(a, b, cmp.Compare)
}

// MergeFunc2 merges two sorted sequences of key-value pairs into a single sorted sequence according to function cmp.
func MergeFunc2[K, V any](a, b iter.Seq2[K, V], cmp func(K, K) int) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		nextB, stopB := iter.Pull2(b)

		defer stopB()

		kb, vb, okb := nextB()

		for ka, va := range a {
			for okb && cmp(ka, kb) > 0 {
				if !yield(kb, vb) {
					return
				}

				kb, vb, okb = nextB()
			}

			if !yield(ka, va) {
				return
			}
		}

		for okb {
			if !yield(kb, vb) {
				return
			}

			kb, vb, okb = nextB()
		}
	}
}

// Reduce applies a function to each element of a sequence, accumulating a result.
func Reduce[S, T any](seq iter.Seq[T], sum S, f func(S, T) S) S {
	for v := range seq {
		sum = f(sum, v)
	}

	return sum
}

// Reduce2 applies a function to each key-value pair of a sequence, accumulating a result.
func Reduce2[S, K, V any](seq iter.Seq2[K, V], sum S, f func(S, K, V) S) S {
	for k, v := range seq {
		sum = f(sum, k, v)
	}

	return sum
}

// Repeat returns a sequence that yields the same value n times.
// If n is negative or zero, it yields the value indefinitely.
func Repeat[T any](v T, n int) (seq iter.Seq[T]) {
	// seq yields the value indefinitely.
	seq = func(yield func(T) bool) {
		for {
			if !yield(v) {
				return
			}
		}
	}

	if n <= 0 {
		return
	}

	return TakeN(seq, n)
}

// Repeat2 returns a sequence that yields the same key-value pair n times.
// If n is negative or zero, it yields the key-value pair indefinitely.
func Repeat2[K, V any](k K, v V, n int) (seq2 iter.Seq2[K, V]) {
	// seq2 yields the key-value pair indefinitely.
	seq2 = func(yield func(K, V) bool) {
		for {
			if !yield(k, v) {
				return
			}
		}
	}

	if n <= 0 {
		return
	}

	return TakeN2(seq2, n)
}

func SeqN[T constraints.Integer](start, end, step T) iter.Seq[T] {
	switch {
	case step == 0, start > end && step > 0, start < end && step < 0:
		return EmptySeq[T]()

	case step < 0:
		return func(yield func(T) bool) {
			for v := start; v > end; v += step {
				if !yield(v) {
					return
				}
			}
		}
	}

	return func(yield func(T) bool) {
		for v := start; v < end; v += step {
			if !yield(v) {
				return
			}
		}
	}
}

// Stride returns a sequence that yields every stride-th element of seq.
func Stride[T any](seq iter.Seq[T], stride int) iter.Seq[T] {
	if stride <= 0 {
		return func(yield func(v T) bool) {}
	}

	return func(yield func(v T) bool) {
		i := 0

		for v := range seq {
			if i%stride == 0 {
				if !yield(v) {
					return
				}
			}

			i++
		}
	}
}

// Stride2 returns a sequence that yields every stride-th key-value pair of seq.
func Stride2[K, V any](seq iter.Seq2[K, V], stride int) iter.Seq2[K, V] {
	if stride <= 0 {
		return func(yield func(k K, v V) bool) {}
	}

	return func(yield func(k K, v V) bool) {
		i := 0

		for k, v := range seq {
			if i%stride == 0 {
				if !yield(k, v) {
					return
				}
			}

			i++
		}
	}
}

// SliceStride returns a sequence that yields every stride-th element of buf.
func SliceStride[T any](buf []T, stride int) iter.Seq[T] {
	return Stride(slices.Values(buf), stride)
}

// TakeN returns a sequence that yields at most the first n elements of seq.
// If n is negative or zero, it yields nothing.
func TakeN[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if n <= 0 || !yield(v) {
				return
			}

			n--
		}
	}
}

// TakeN2 returns a sequence that yields at most the first n key-value pairs of seq.
// If n is negative or zero, it yields nothing.
func TakeN2[K, V any](seq iter.Seq2[K, V], n int) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range seq {
			if n <= 0 || !yield(k, v) {
				return
			}

			n--
		}
	}
}

// ZipX are consistent with python semantics. rsc sees things differently and is probably right.
// Please be aware there's a chance that this will be changed in the future to match rsc's
// proposal.

// Zip2 returns a sequence that yields keys and values from pairs of iterators k and v.
// If k and v have different lengths, the resulting sequence will be as long as the shorter of
// the two.
func Zip2[K, V any](k iter.Seq[K], v iter.Seq[V]) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		nextB, stopB := iter.Pull(v)

		defer stopB()

		for va := range k {
			vb, okb := nextB()

			if !okb || !yield(va, vb) {
				return
			}
		}
	}
}

// SliceZip2 returns a sequence that yields pairs of elements from slices k and v.
func SliceZip2[K, V any](k []K, v []V) iter.Seq2[K, V] {
	return Zip2(slices.Values(k), slices.Values(v))
}
