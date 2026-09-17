// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

import "testing"

func TestWinBufferReadWindowWraps(t *testing.T) {
	wb := &wbuf[int]{
		buf:     []int{0, 1, 2, 3, 4, 5, 6, 7},
		cap:     8,
		winsz:   5,
		hopsz:   2,
		wpos:    0,
		wcnt:    8,
		rpos:    6,
		pending: 8,
		filled:  8,
	}

	out := make([]int, wb.winsz)
	n, err := wb.readWindow(out)

	if err != nil {
		t.Fatalf("ReadWindow error: %v", err)
	}

	if n != wb.winsz {
		t.Fatalf("ReadWindow returned %d, want %d", n, wb.winsz)
	}

	want := []int{6, 7, 0, 1, 2}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("window[%d] = %d, want %d", i, out[i], want[i])
		}
	}
}

func TestWinBufferAvailableAfterRead(t *testing.T) {
	wb, err := newbuf[int](8, 4, 2)
	if err != nil {
		t.Fatalf("NewWinBuffer error: %v", err)
	}

	wb.write(0, 1, 2, 3)
	out := make([]int, 4)
	if _, err := wb.readWindow(out); err != nil {
		t.Fatalf("ReadWindow error: %v", err)
	}

	wb.write(4)
	if got := wb.available(); got != 3 {
		t.Fatalf("Available after one write = %d, want 3", got)
	}

	wb.write(5)
	if got := wb.available(); got != 4 {
		t.Fatalf("Available after two writes = %d, want 4", got)
	}

	if _, err := wb.readWindow(out); err != nil {
		t.Fatalf("ReadWindow error: %v", err)
	}
	if got := wb.available(); got != 2 {
		t.Fatalf("Available after second read = %d, want 2", got)
	}
}
