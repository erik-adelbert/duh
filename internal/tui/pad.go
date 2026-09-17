// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import "strings"

// Pre-compute common space strings
var spaceCache = strings.Repeat(" ", 128) // Pre-allocate max width

func Mkpad(n int) string {
	if n <= len(spaceCache) {
		return spaceCache[:n]
	}

	return strings.Repeat(" ", n)
}
