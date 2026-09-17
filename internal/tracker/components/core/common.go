// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package core

import "golang.org/x/exp/constraints"

type Span[T constraints.Integer] struct{ Start, End T }

type Balance[T any] struct{ Vol, Pan T }

type Stereo[T any] struct{ Left, Right T }

type Voice[T any] struct {
	Balance[T]
	Pitch T
}

type LoopSustain[T constraints.Integer] struct {
	Loop    Span[T]
	Sustain Span[T]
}
