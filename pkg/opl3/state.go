// duh OPL3 emulator package
//
// Based on Nuked OPL3 by Nuke.YKT.
//
// Original:
// Copyright (C) 2013-2020 Nuke.YKT
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package opl3

import "sync"

type State struct {
	TimerStatus uint8
	Registers
}

func (st *State) Free() {
	putState(st)
}

var stPool = sync.Pool{
	New: func() any {
		return &State{}
	},
}

func getState() *State {
	return stPool.Get().(*State)
}

func putState(st *State) {
	st.TimerStatus = 0
	clear(st.Registers[:])
	stPool.Put(st)
}
