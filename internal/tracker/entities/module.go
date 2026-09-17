package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
)

type Module struct {
	mod.Flags
	mod.Metadata
	mod.Playback
	mod.MixBus
	Orders      []id.ID
	Patterns    []id.ID
	Samples     []id.ID
	Instruments []id.ID
	Channels    []id.ID
	Plugins     []id.ID
	OutBuffer   id.ID
}

func AddModule(w *ecs.World, m Module) id.ID {
	mid := ecs.NewEntity(w)

	ecs.SetComponent(w, "", mid, m.Flags)
	ecs.SetComponent(w, "", mid, m.Metadata)
	ecs.SetComponent(w, "", mid, m.Playback)
	ecs.SetComponent(w, "", mid, m.MixBus)
	ecs.SetComponent(w, "Orders", mid, m.Orders)
	ecs.SetComponent(w, "Patterns", mid, m.Patterns)
	ecs.SetComponent(w, "Samples", mid, m.Samples)
	ecs.SetComponent(w, "Instruments", mid, m.Instruments)
	ecs.SetComponent(w, "Channels", mid, m.Channels)
	ecs.SetComponent(w, "Plugins", mid, m.Plugins)
	ecs.SetComponent(w, "OutBuffer", mid, m.OutBuffer)

	return mid
}

func GetModule(w *ecs.World, mid id.ID) (Module, bool) {
	var m Module

	if !ecs.GetComponent(w, auto, mid, &m.Flags) ||
		!ecs.GetComponent(w, auto, mid, &m.Metadata) ||
		!ecs.GetComponent(w, auto, mid, &m.Playback) ||
		!ecs.GetComponent(w, auto, mid, &m.MixBus) ||
		!ecs.GetComponent(w, "Orders", mid, &m.Orders) ||
		!ecs.GetComponent(w, "Patterns", mid, &m.Patterns) ||
		!ecs.GetComponent(w, "Samples", mid, &m.Samples) ||
		!ecs.GetComponent(w, "Instruments", mid, &m.Instruments) ||
		!ecs.GetComponent(w, "Channels", mid, &m.Channels) ||
		!ecs.GetComponent(w, "Plugins", mid, &m.Plugins) ||
		!ecs.GetComponent(w, "OutBuffer", mid, &m.OutBuffer) {
		return m, false
	}

	return m, true
}

func DeleteModule(w *ecs.World, mid id.ID) {
	ecs.RemoveEntity(w, mid)
}
