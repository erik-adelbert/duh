// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package id

import "iter"

type ID = uint32

func NewIDPuller(start ID) (next func() (ID, bool), stop func()) {
	nextID := max(start, ID(1))

	it := func(yield func(ID) bool) {
		for {
			id := nextID
			nextID++

			if !yield(id) {
				return
			}
		}
	}

	return iter.Pull(it)
}

func Null() ID { return ID(0) }
