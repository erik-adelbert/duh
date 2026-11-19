// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/erik-adelbert/duh/pkg/opl3"
)

func TestOPL3StateInitialAndUpdates(t *testing.T) {
	view := NewOPL3State()
	var scratch, rendered strings.Builder
	scratch.WriteString("discard this")
	view.Update(&scratch, opl3.State{})
	view.Render(&rendered, "")
	for id := range opl3.ChannelCount {
		for op := range 2 {
			label := fmt.Sprintf("Ch%02d-%d:", id, op)
			if !strings.Contains(rendered.String(), label) {
				t.Fatalf("initial frame missing %s: %q", label, rendered.String())
			}
		}
	}
	if strings.Contains(rendered.String(), "discard this") || !strings.Contains(rendered.String(), "Timer0") {
		t.Fatal("initial timer display is missing or contaminated")
	}

	var next opl3.State
	next.TimerStatus = 0xc0
	next.Registers[1][0x20] = 7
	next.Registers[1][0x05] = 1
	view.Update(&scratch, next)
	var expected strings.Builder
	timerStatus(&expected, next.TimerStatus)
	if view.cache[TimerState] != expected.String() {
		t.Fatal("timer did not render the new hardware status")
	}
	expected.Reset()
	channel(&expected, next.Channel(9))
	if view.cache[Chan0+9] != expected.String() {
		t.Fatal("bank 1 channel did not update")
	}
	expected.Reset()
	status(&expected, next.Status())
	if view.cache[Status] != expected.String() {
		t.Fatal("status did not update")
	}
}

func TestOPL3StatusFlagAlignment(t *testing.T) {
	old := NO_COLOR
	NO_COLOR = false
	t.Cleanup(func() { NO_COLOR = old })
	var out strings.Builder
	status(&out, opl3.Flags(1<<opl3.CSM))
	if !strings.Contains(out.String(), Green(Bold("CSM"))) ||
		strings.Contains(out.String(), Green(Bold("WF"))) {
		t.Fatalf("CSM status not aligned: %q", out.String())
	}
}

func TestOPL3HeaderAlignment(t *testing.T) {
	view := NewOPL3State()
	var scratch, rendered strings.Builder
	view.Update(&scratch, opl3.State{})
	view.Render(&rendered, "  ")

	ansi := regexp.MustCompile("\x1b\\[[0-9;]*m")
	lines := strings.Split(ansi.ReplaceAllString(rendered.String(), ""), "\n")
	header, operator := lines[2], lines[3]
	fields := strings.Fields(operator)
	starts := make([]int, len(fields))
	offset := 0
	for i, field := range fields {
		starts[i] = offset + strings.Index(operator[offset:], field)
		offset = starts[i] + len(field)
	}
	for _, field := range []struct {
		label string
		index int
	}{
		{"ml", 5}, {"k", 6}, {"tl", 7},
		{"a", 8}, {"d", 9}, {"s", 10},
		{"r", 11}, {"o", 12}, {"freq", 13},
	} {
		start := strings.Index(header, field.label)
		if start != starts[field.index] {
			t.Errorf("%s header starts at %d, operator field starts at %d; header %q operator %q", field.label, start, starts[field.index], header, operator)
		}
	}
}
