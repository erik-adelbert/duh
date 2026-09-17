// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Functions and types for Instrument entities (sample/synth definitions).
package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
	env "github.com/erik-adelbert/duh/internal/tracker/components/envelope"
	ins "github.com/erik-adelbert/duh/internal/tracker/components/instrument"
)

type Instrument struct {
	ModuleID id.ID

	core.Balance[uint16]
	env.Envelope
	ins.Filter
	ins.Metadata
	ins.Midi
	ins.NoteBehavior
	ins.Mappings
	ins.PitchPan

	Params         core.Voice[ins.Params]
	Swing          core.Balance[uint8]
	DefaultPanning uint8
	GlobalVolume   uint8
	FadeOut        uint8
}

func AddInstrument(w *ecs.World, i Instrument) id.ID {
	iid := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", iid, i.ModuleID)

	ecs.SetComponent(w, auto, iid, i.Balance)
	ecs.SetComponent(w, auto, iid, i.Envelope)
	ecs.SetComponent(w, auto, iid, i.Filter)
	ecs.SetComponent(w, auto, iid, i.Metadata)
	ecs.SetComponent(w, auto, iid, i.Midi)
	ecs.SetComponent(w, auto, iid, i.NoteBehavior)
	ecs.SetComponent(w, auto, iid, i.Mappings)
	ecs.SetComponent(w, auto, iid, i.PitchPan)
	ecs.SetComponent(w, auto, iid, i.Params)
	ecs.SetComponent(w, auto, iid, i.Swing)
	ecs.SetComponent(w, "DefaultPanning", iid, i.DefaultPanning)
	ecs.SetComponent(w, "GlobalVolume", iid, i.GlobalVolume)
	ecs.SetComponent(w, "FadeOut", iid, i.FadeOut)

	return iid
}

func GetInstrument(w *ecs.World, iid id.ID) (Instrument, bool) {
	var i Instrument

	if !ecs.GetComponent(w, "ModuleID", iid, &i.ModuleID) ||
		!ecs.GetComponent(w, auto, iid, &i.Balance) ||
		!ecs.GetComponent(w, auto, iid, &i.Envelope) ||
		!ecs.GetComponent(w, auto, iid, &i.Filter) ||
		!ecs.GetComponent(w, auto, iid, &i.Metadata) ||
		!ecs.GetComponent(w, auto, iid, &i.Midi) ||
		!ecs.GetComponent(w, auto, iid, &i.NoteBehavior) ||
		!ecs.GetComponent(w, auto, iid, &i.Mappings) ||
		!ecs.GetComponent(w, auto, iid, &i.PitchPan) ||
		!ecs.GetComponent(w, auto, iid, &i.Params) ||
		!ecs.GetComponent(w, auto, iid, &i.Swing) ||
		!ecs.GetComponent(w, "DefaultPanning", iid, &i.DefaultPanning) ||
		!ecs.GetComponent(w, "GlobalVolume", iid, &i.GlobalVolume) ||
		!ecs.GetComponent(w, "FadeOut", iid, &i.FadeOut) {
		return i, false
	}

	return i, true
}

func DeleteInstrument(w *ecs.World, iID id.ID) {
	ecs.RemoveEntity(w, iID)
}
