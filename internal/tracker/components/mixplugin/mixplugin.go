// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mixplugin

import "github.com/erik-adelbert/duh/internal/tracker/components/core"

type State struct {
	Flags
	VolDecay core.Stereo[int32]
	Out      core.Stereo[[]float32]
	Buf      []int32
}

type Infos struct {
	ID1, ID2 uint32
	Name     string
	Library  string
	Routing  struct {
		In, Out uint32
		Infos   [4]uint32
	}
}

type Mixer interface {
	AddRef() int
	Release() int
	Init(freq uint, reset bool)
	Save()
	Load()
	Mix(outLeft, outRight []float32)
	MidiSend(msg uint32)
	MidiCommand(channel, program, note, volume uint32)
}

type Plugin struct {
	Data any
	Size int
}
