// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package help

import (
	"reflect"
	"slices"
	"testing"
)

func TestChunks(t *testing.T) {
	got := slices.Collect(Chunks([]int{1, 2, 3, 4, 5}, 2))
	want := [][]int{{1, 2}, {3, 4}, {5}}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AllChunks() = %v, want %v", got, want)
	}
}

func TestChunks_Empty(t *testing.T) {
	got := slices.Collect(Chunks([]int{}, 3))

	if len(got) != 0 {
		t.Fatalf("AllChunks() with empty input = %v, want empty", got)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name string
		v    int
		l    int
		r    int
		want int
	}{
		{name: "below lower bound", v: -3, l: 0, r: 10, want: 0},
		{name: "within bounds", v: 5, l: 0, r: 10, want: 5},
		{name: "above upper bound", v: 12, l: 0, r: 10, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Clamp(tt.v, tt.l, tt.r)

			if got != tt.want {
				t.Fatalf("Clamp(%d, %d, %d) = %d, want %d", tt.v, tt.l, tt.r, got, tt.want)
			}
		})
	}
}
