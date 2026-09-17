// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tui

import (
	"fmt"
	"strings"

	"github.com/erik-adelbert/duh/pkg/opl3"
)

const (
	TimerState = iota
	Status
	Header
	Chan0
	Chan1
	Chan2
	Chan3
	Chan4
	Chan5
	Chan6
	Chan7
)

type OPL3State struct {
	cache [11]string

	opl3.State
}

func NewOPL3State() *OPL3State {
	const npad = 23

	pads := Mkpad(npad)
	header := Bold("ml k tl a  d  s  r  o  freq    fbk    wave")

	return &OPL3State{
		cache: [11]string{
			Header: fmt.Sprint(pads, header),
		},
	}
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

	for i := range 8 {
		sb.WriteString(pads)
		sb.WriteString(strings.ReplaceAll(st.cache[Chan0+i], "\n", "\n"+pads))
		if i < 7 {
			sb.WriteByte('\n')
		}
	}
}

func (st *OPL3State) Update(sb *strings.Builder, nxt opl3.State) {

	if st.TimerStatus != nxt.TimerStatus {
		timerStatus(sb, st.TimerStatus)
		st.cache[TimerState] = sb.String()
	}

	nxtStatus := nxt.Status()
	curStatus := st.Status()

	if curStatus != nxtStatus {
		sb.Reset()

		status(sb, nxtStatus)
		st.cache[Status] = sb.String()
	}

	for i := range 8 {
		nxtCh := nxt.Channel(i)
		curCh := st.Channel(i)

		if curCh != nxtCh {
			sb.Reset()

			channel(sb, nxtCh)
			st.cache[Chan0+i] = sb.String()
		}
	}

	st.State = nxt
}

func bit(bit int, u8 uint8) bool {
	return u8&(1<<bit) != 0
}

func timerStatus(sb *strings.Builder, tstat uint8) {
	timer0 := bit(0, tstat)
	timer1 := bit(1, tstat)
	expired := bit(2, tstat)

	t0 := DarkGrey("Timer0")
	t1 := DarkGrey("Timer1")

	color := Green
	if expired {
		color = Red
	}

	if timer0 {
		t0 = color(t0)
	}

	if timer1 {
		t1 = color(t1)
	}

	fmt.Fprintf(sb, "%s %s", t0, t1)
}

func status(sb *strings.Builder, stat opl3.Flags) {
	pads := Mkpad(7)

	var flags = [opl3.NOPL3FLAG + 1]string{
		pads,

		"CSM", "WF", "HH", "CYM", "TOM", "SD",
		"BD", "DRUM", "VIBI", "AMI", "OPL3", "4OP",
	}

	for i, f := range flags[1:] {
		flags[i+1] = greenFLag(stat.Has(i + 1)).render(f)
	}
	sb.WriteString(strings.Join(flags[:], " "))
}

func channel(sb *strings.Builder, ch opl3.OPL3Channel) {
	fmt.Fprintf(sb, "Ch%d-0: ", ch.ID)
	op(ch.Ops[0]).frender(sb)
	fmt.Fprintf(sb, "\nCh%d-1: ", ch.ID)
	op(ch.Ops[1]).frender(sb)
}

type op opl3.OP

func (o op) frender(sb *strings.Builder) {

	opflags := o.OpFlags

	flags := []string{
		opl3.KSR: "KSR",
		opl3.EGT: "EGT",
		opl3.VIB: "VIBI",
		opl3.AM:  "AM",

		opl3.KEY: "KEY",
		opl3.ALG: "ALG",
	}

	for i := range flags {
		if flags[i] == "" {
			continue
		}

		if opflags.Has(opl3.OpFlags(i)) {
			flags[i] = Green(Bold(flags[i]))
		}
	}

	state := o.OpState

	fmt.Fprintf(sb, "%s %s %s %s %02d %01d %02d %02d %02d %02d %02d %02d %04d %s %01d %s %v",
		flags[opl3.AM], flags[opl3.VIB], flags[opl3.EGT], flags[opl3.KSR],
		state[opl3.ML], state[opl3.K], state[opl3.TL],
		state[opl3.A], state[opl3.D], state[opl3.S],
		state[opl3.R], state[opl3.OCT], o.FQ,
		flags[opl3.KEY], state[opl3.FB], flags[opl3.ALG], o.WF,
	)
}

type greenFLag bool

func (b greenFLag) render(s string) string {
	if b {
		return Green(Bold(s))
	}
	return s
}
