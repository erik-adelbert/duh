// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import "strings"

type Screener func(sb *strings.Builder)

type Screen struct {
	strings.Builder
	update Screener
}

func NewScreen(f Screener, dynamic bool) *Screen {
	var (
		sb     strings.Builder
		update Screener
	)

	if !dynamic {
		f(&sb)
	} else {
		update = f
	}

	return &Screen{
		Builder: sb,
		update:  update,
	}
}

func (s *Screen) Render() string {
	if s.update != nil {
		s.update(&s.Builder)

		defer s.Reset()
	}

	return s.String()
}
