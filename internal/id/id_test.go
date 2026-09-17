// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package id

import (
	"testing"
)

func TestNewIDPuller_DefaultStart(t *testing.T) {
	next, stop := NewIDPuller(0)

	defer stop()

	id1, ok1 := next()
	id2, ok2 := next()
	id3, ok3 := next()

	if !ok1 || !ok2 || !ok3 {
		t.Fatal("expected all IDs to be generated successfully")
	}

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Errorf("expected IDs 1,2,3; got %d,%d,%d", id1, id2, id3)
	}
}

func TestNewIDPuller_CustomStart(t *testing.T) {
	next, stop := NewIDPuller(100)

	defer stop()

	id1, ok1 := next()
	id2, ok2 := next()

	if !ok1 || !ok2 {
		t.Fatal("expected IDs to be generated successfully")
	}

	if id1 != 100 || id2 != 101 {
		t.Errorf("expected IDs 100,101; got %d,%d", id1, id2)
	}
}

func TestNewIDPuller_Stop(t *testing.T) {
	next, stop := NewIDPuller(0)
	_, ok1 := next()

	stop()

	id2, ok2 := next()

	if !ok1 {
		t.Fatal("expected IDs to be generated successfully")
	}

	if ok2 {
		t.Errorf("expected IDs not to be generated successfully after stop, but got %d", id2)
	}
}

func TestNull(t *testing.T) {
	if Null() != 0 {
		t.Errorf("expected Null() to return 0, got %d", Null())
	}
}
