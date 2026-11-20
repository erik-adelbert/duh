// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestSpectroRenderReference(t *testing.T) {
	old := NO_COLOR
	t.Cleanup(func() { NO_COLOR = old })
	for _, tt := range []struct {
		name    string
		bands   int
		emoji   bool
		noColor bool
		want    string
	}{
		{"color", 64, false, false, "40b569a38b9851f19ab2bc620e5882908bff81acdf3f1b236882c6f2a72af1ae"},
		{"no-color", 64, false, true, "91bb6136b1580944a5193b5a03dc5ba3015d86131bc36e02f7c979163b6e43e8"},
		{"emoji", 64, true, false, "ec8426bcdb886e4c24fdbe01566782d39ea032cf863e58342f325706ead82694"},
		{"odd-bands", 7, false, false, "a4d163024af236cfc221d7bc15b463933d1ea2db43c0e32da8de937698062e63"},
		{"many-bands", 96, false, false, "4444b9de6ff4c8da38392213ec7e59535e055bf5f433c5feee39f5a366f16e1b"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			NO_COLOR = tt.noColor
			sp, err := NewSpectro(48000, 50, 20000, tt.bands, tt.emoji)
			if err != nil {
				t.Fatal(err)
			}
			for i := range sp.val.Levels {
				sp.val.Levels[i] = float64((i*37)%101) / 100
				sp.val.Peaks[i] = float64((i*71)%101) / 100
			}
			got := fmt.Sprintf("%x", sha256.Sum256([]byte(sp.Render("  "))))
			if got != tt.want {
				t.Fatalf("render hash = %s, want %s", got, tt.want)
			}
		})
	}
}

func BenchmarkSpectroUpdate(b *testing.B) {
	sp, err := NewSpectro(48000, 50, 20000, 64, false)
	if err != nil {
		b.Fatal(err)
	}

	var out strings.Builder
	b.ReportAllocs()

	for i := 0; b.Loop(); i++ {
		for j := range 1470 {
			x := float64(i*1470+j) * 2 * math.Pi / 48000
			sp.Addf64(0.3*math.Sin(x*220) + 0.2*math.Sin(x*1200))
		}
		out.Reset()
		sp.FRender(&out, "  ")
	}
}

func TestSpectroColorChanges(t *testing.T) {
	oldNoColor, oldRed := NO_COLOR, Red
	t.Cleanup(func() { NO_COLOR, Red = oldNoColor, oldRed })
	sp, err := NewSpectro(48000, 50, 20000, 64, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range sp.val.Levels {
		sp.val.Levels[i] = 1
	}
	NO_COLOR = false
	Red = mkColorizer("custom-red")
	if got := sp.Render(""); !strings.Contains(got, "custom-red") {
		t.Fatal("custom color was not rendered")
	}
	Red = mkColorizer("updated-red")
	if got := sp.Render(""); !strings.Contains(got, "updated-red") || strings.Contains(got, "custom-red") {
		t.Fatal("updated color was not rendered")
	}
	NO_COLOR = true
	if got := sp.Render(""); strings.Contains(got, "updated-red") {
		t.Fatal("color remained enabled")
	}
}

func TestSpectroRenderAllocations(t *testing.T) {
	old := NO_COLOR
	NO_COLOR = false
	t.Cleanup(func() { NO_COLOR = old })
	sp, err := NewSpectro(48000, 50, 20000, 64, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range sp.val.Levels {
		sp.val.Levels[i] = float64((i*37)%101) / 100
		sp.val.Peaks[i] = float64((i*71)%101) / 100
	}
	allocs := testing.AllocsPerRun(100, func() {
		var out strings.Builder
		sp.FRender(&out, "  ")
	})
	if allocs > 40 {
		t.Fatalf("spectrum render allocated %.0f times, want at most 40", allocs)
	}
}
