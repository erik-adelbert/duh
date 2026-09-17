// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"strings"

	"github.com/erik-adelbert/duh/internal/help"
)

var RedPeak = Red("|")

func Slider(value float64, width int) string {
	value = help.Clamp(value, 0, 1)

	bar := strings.Repeat("-", width)
	pos := int(value * float64(width-1))
	out := bar[:pos] + "▉" + bar[pos+1:]

	if value == 0 {
		return Red(out)
	}

	return Green(out)
}
