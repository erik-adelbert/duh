// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"time"
)

func TimerString(d time.Duration) string {
	var buf [15]byte // "HH:MM:SS.CC" = 11, "MM:SS.CC" = 8

	ms := d.Milliseconds()
	if ms < 0 {
		ms = -ms // or handle sign explicitly
	}

	hundredths := (ms / 10) % 100
	s := ms / 1000
	sec := s % 60
	m := s / 60
	min := m % 60
	hr := m / 60

	b := buf[:0]
	if hr > 0 {
		b = append2(b, hr)
		b = append(b, ':')
	}
	b = append2(b, min)
	b = append(b, ':')
	b = append2(b, sec)
	b = append(b, '.')
	b = append2(b, hundredths)
	return string(b) // one alloc (the string)
}

func append2(b []byte, v int64) []byte {
	return append(b, byte('0'+v/10), byte('0'+v%10))
}
