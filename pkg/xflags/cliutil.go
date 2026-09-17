// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// package xflags provides 6 CLI helper functions that are
// working well with go flags in CLI applications.
package xflags

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
)

var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "Enable verbose output")
}

// MkDie creates a function that prints an error message and exits the
// program with the specified exit code.
func MkDie(tty *os.File) func(base error, reason string, code int) {
	return func(base error, reason string, code int) {
		if base == nil {
			return
		}

		format := fmt.Sprintf("%s: %%v\n", os.Args[0])

		if reason != "" {
			format = fmt.Sprintf("%s: %s: %%v\n", os.Args[0], reason)
		}

		_, _ = fmt.Fprintf(tty, format, base)
		_, _ = fmt.Fprintf(tty, "Try '%s -h' for help\n", os.Args[0])

		os.Exit(code)
	}
}

// MkVerbosef creates a function that prints verbose output
// to the specified TTY.
func MkVerbosef(tty *os.File) func(format string, a ...any) {
	return func(format string, a ...any) {
		if !verbose {
			return
		}

		fmt.Fprintf(tty, format, a...)
	}
}

// MkPrintf creates a function that prints output
// to the specified TTY.
func MkPrintf(tty *os.File) func(a ...any) {
	return func(a ...any) {
		fmt.Fprint(tty, a...)
	}
}

// MkPrint creates a function that prints output
// to the specified TTY without formatting.
func MkPrint(tty *os.File) func(a ...any) {
	return func(a ...any) {
		fmt.Fprint(tty, a...)
	}
}

// SigHandle sets up a signal handler for the specified signals.
// It returns a channel that receives the signals and a function
// to stop handling them.
func SigHandle(s ...os.Signal) (<-chan os.Signal, func()) {
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, s...)

	return sigs, func() {
		signal.Stop(sigs)
	}
}

// OpenTTY attempts to open the TTY device for writing.
// If it fails, it falls back to the specified file.
// It returns the TTY file and a function to close it.
func OpenTTY(fallback *os.File) (tty *os.File, close func()) {
	x, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)

	if err != nil {
		tty = fallback
		return tty, func() {}
	}

	tty = x
	return tty, func() {
		_ = tty.Close()
	}
}
