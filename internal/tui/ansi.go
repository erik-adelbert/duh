// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"os"

	"github.com/charmbracelet/x/term"
)

var NO_COLOR = NoColor()

func AltScreen(on bool) string {
	if on {
		return ANSIAltScreenOn + ANSIClear + ANSIHome + ANSICursorOff
	}

	return ANSIAltScreenOff + ANSICursorOn
}

const (
	ANSIRed      = "\033[31m"
	ANSIGreen    = "\033[32m"
	ANSIYellow   = "\033[33m"
	ANSIBlue     = "\033[34m"
	ANSIMagenta  = "\033[35m"
	ANSICyan     = "\033[36m"
	ANSIDarkGrey = "\033[48;2;40;40;40m"
	ANSIReset    = "\033[0m"

	ANSIBlackOnDarkGrey = "\x1b[30;100m"

	ANSIInvertedColor = "\033[7m"

	ANSICLS   = "\033[H\033[2J"
	ANSIHome  = "\033[H"
	ANSIClear = "\033[2J"

	ANSICursorOff = "\033[?25l"
	ANSICursorOn  = "\033[?25h"

	ANSIBold = "\033[1m"
)

const (
	ANSIAltScreenOn  = "\033[?1049h"
	ANSIAltScreenOff = "\033[?1049l"
)

func barColorBand(i, w int) Colorizer {
	switch {
	case i < w/4:
		return Red
	case i < w/2:
		return Yellow
	case i < 3*w/4:
		return Green
	}

	return Cyan
}

func barColorVu(i, w int) Colorizer {
	rat := float64(i) / float64(w)

	switch {
	case rat > 0.87:
		return Red
	case rat > 0.55:
		return Yellow
	default:
		return Green
	}
}

func Invert(s string) string {
	if NO_COLOR {
		return s
	}

	return ANSIInvertedColor + s + ANSIReset
}

func Bold(s string) string {
	// if NO_COLOR {
	// 	return s
	// }

	return ANSIBold + s + ANSIReset
}

type Colorizer func(string) string

func mkColorizer(color string) Colorizer {
	return func(s string) string {
		if NO_COLOR {
			return s
		}

		return color + s + ANSIReset
	}
}

func SysColor(s string) string {
	return s
}

var (
	Red    = mkColorizer(ANSIRed)
	Green  = mkColorizer(ANSIGreen)
	Blue   = mkColorizer(ANSIBlue)
	Yellow = mkColorizer(ANSIYellow)
	Cyan   = mkColorizer(ANSICyan)

	DarkGrey    = mkColorizer(ANSIDarkGrey)
	InvDarkGrey = mkColorizer(ANSIBlackOnDarkGrey)

	// Invert = mkColorizer(ANSIInvertedColor)
)

// NoColor checks if the terminal should disable color output.
func NoColor() bool {
	return os.Getenv("NO_COLOR") != "" ||
		os.Getenv("TERM") == "dumb" ||
		!term.IsTerminal(os.Stdout.Fd())
}
