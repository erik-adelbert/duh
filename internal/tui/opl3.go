// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/erik-adelbert/duh/pkg/opl3"
)

const (
	TimerState = iota
	Status
	Header
	Chan0
)

// opFlagLabels is the fixed set of op-flag labels, indexed to match
// opl3.OpFlags. Immutable; never mutated.
var opFlagLabels = [...]string{
	opl3.KSR: "KSR",
	opl3.EGT: "EGT",
	opl3.VIB: "VIBI",
	opl3.AM:  "AM",
	opl3.KEY: "KEY",
	opl3.ALG: "ALG",
}

// opFlagOn is opFlagLabels with each label pre-colored for the "set" case.
// Built once at init; frender picks between opFlagLabels and opFlagOn with
// zero per-call allocation.
var opFlagOn = func() (on [len(opFlagLabels)]string) {
	for i, s := range opFlagLabels {
		if s != "" {
			on[i] = Green(Bold(s))
		}
	}
	return on
}()

// statusLabels is the fixed status-flag label set, indexed to match
// opl3.Flags. Index 0 is a padding entry (unused / never set by callers).
var statusLabels = [opl3.NOPL3FLAG + 1]string{
	"",
	"CSM", "WF", "HH", "CYM", "TOM", "SD",
	"BD", "DRUM", "VIBI", "AMI", "OPL3", "4OP",
}

// statusOn is statusLabels pre-colored for the "set" case.
var statusOn = func() (on [len(statusLabels)]string) {
	for i, s := range statusLabels {
		if s != "" {
			on[i] = Invert(Green(Bold(s)))
		}
	}
	return on
}()

type OPL3State struct {
	cache       [Chan0 + opl3.ChannelCount]string
	initialized bool

	opl3.State
}

func NewOPL3State() *OPL3State {
	const npad = 24

	pads := Mkpad(npad)
	header := Bold("ml k tl a  d  s  r  o  freq    fbk    wave")

	st := &OPL3State{}
	st.cache[Header] = pads + header
	return st
}

func (st *OPL3State) Render(sb *strings.Builder, pads string) {
	sb.WriteString(pads)
	sb.WriteString(st.cache[TimerState])
	sb.WriteByte('\n')

	sb.WriteString(pads)
	sb.WriteString(st.cache[Status])
	sb.WriteByte('\n')

	sb.WriteString(pads)
	sb.WriteString(st.cache[Header])
	sb.WriteByte('\n')

	for i := range opl3.ChannelCount {
		sb.WriteString(pads)
		sb.WriteString(strings.ReplaceAll(st.cache[Chan0+i], "\n", "\n"+pads))

		if i < opl3.ChannelCount-1 {
			sb.WriteByte('\n')
		}
	}
}

func (st *OPL3State) Update(sb *strings.Builder, nxt opl3.State) {
	if !st.initialized || st.TimerStatus != nxt.TimerStatus {
		sb.Reset()
		timerStatus(sb, nxt.TimerStatus)
		st.cache[TimerState] = sb.String()
	}

	nxtStatus := nxt.Status()
	curStatus := st.Status()

	if !st.initialized || curStatus != nxtStatus {
		sb.Reset()
		status(sb, nxtStatus)
		st.cache[Status] = sb.String()
	}

	for i := range opl3.ChannelCount {
		nxtCh := nxt.Channel(i)
		curCh := st.Channel(i)

		if !st.initialized || curCh != nxtCh {
			sb.Reset()
			channel(sb, nxtCh)
			st.cache[Chan0+i] = sb.String()
		}
	}

	st.State = nxt
	st.initialized = true
}

func bit(bit int, u8 uint8) bool {
	return u8&(1<<bit) != 0
}

func timerStatus(sb *strings.Builder, tstat uint8) {
	timer0 := bit(6, tstat)
	timer1 := bit(5, tstat)
	expired := bit(7, tstat)

	t0 := "Timer0"
	t1 := "Timer1"

	color := Green
	if expired {
		color = Red
	}

	if timer0 {
		t0 = color(t0)
	} else {
		t0 = DarkGrey(t0)
	}

	if timer1 {
		t1 = color(t1)
	} else {
		t1 = DarkGrey(t1)
	}

	sb.WriteString(t0)
	sb.WriteByte(' ')
	sb.WriteString(t1)
}

func status(sb *strings.Builder, stat opl3.Flags) {
	pads := Mkpad(7)

	sb.WriteString(pads)
	for i := 1; i < len(statusLabels); i++ {
		if i > 1 {
			sb.WriteByte(' ')
		}
		if stat.Has(i - 1) {
			sb.WriteString(statusOn[i])
		} else {
			sb.WriteString(statusLabels[i])
		}
	}
}

func channel(sb *strings.Builder, ch opl3.OPL3Channel) {
	sb.WriteString("Ch")
	writeUint2(sb, uint(ch.ID))
	sb.WriteString("-0: ")
	op(ch.Ops[0]).frender(sb)
	sb.WriteString("\nCh")
	writeUint2(sb, uint(ch.ID))
	sb.WriteString("-1: ")
	op(ch.Ops[1]).frender(sb)
}

type op opl3.OP

func (o op) frender(sb *strings.Builder) {
	opflags := o.OpFlags
	state := o.OpState

	// Flags: pick pre-colored or plain per the set bits. No allocation.
	f := func(i opl3.OpFlags) string {
		if opflags.Has(i) {
			return opFlagOn[i]
		}
		return opFlagLabels[i]
	}

	sb.WriteString(f(opl3.AM))
	sb.WriteByte(' ')
	sb.WriteString(f(opl3.VIB))
	sb.WriteByte(' ')
	sb.WriteString(f(opl3.EGT))
	sb.WriteByte(' ')
	sb.WriteString(f(opl3.KSR))
	sb.WriteByte(' ')

	writeUint2(sb, uint(state[opl3.ML]))
	sb.WriteByte(' ')
	writeUint1(sb, uint(state[opl3.K]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.TL]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.A]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.D]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.S]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.R]))
	sb.WriteByte(' ')
	writeUint2(sb, uint(state[opl3.OCT]))
	sb.WriteByte(' ')
	writeUint4(sb, uint(o.FQ))
	sb.WriteByte(' ')

	sb.WriteString(f(opl3.KEY))
	sb.WriteByte(' ')
	writeUint1(sb, uint(state[opl3.FB]))
	sb.WriteByte(' ')
	sb.WriteString(f(opl3.ALG))
	sb.WriteByte(' ')

	// WF is a plain value (not a flag); use fmt only for the general case.
	// If WF is a small int/uint, writeUint* is cheaper; keep %v for safety.
	fmt.Fprint(sb, o.WF)
}

// The %02d / %01d / %04d helpers below avoid fmt's format-string parsing
// and pp-pool overhead for the integer fields. They write directly into sb.

func writeUint1(sb *strings.Builder, v uint) {
	if v < 10 {
		sb.WriteByte(byte('0' + v))
		return
	}
	sb.WriteString(strconv.FormatUint(uint64(v), 10))
}

func writeUint2(sb *strings.Builder, v uint) {
	switch {
	case v < 10:
		sb.WriteByte('0')
		sb.WriteByte(byte('0' + v))
	case v < 100:
		sb.WriteByte(byte('0' + v/10))
		sb.WriteByte(byte('0' + v%10))
	default:
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
	}
}

func writeUint4(sb *strings.Builder, v uint) {
	switch {
	case v < 10:
		sb.WriteString("000")
	case v < 100:
		sb.WriteString("00")
	case v < 1000:
		sb.WriteByte('0')
	default:
		sb.WriteString(strconv.FormatUint(uint64(v), 10))
		return
	}
	sb.WriteString(strconv.FormatUint(uint64(v), 10))
}
