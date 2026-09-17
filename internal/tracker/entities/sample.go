// Functions and types for Sample entities (audio data).
package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/sample"
)

type Sample struct {
	ModuleID id.ID

	sample.Data
	sample.Flags
	sample.Infos
	sample.Volume
	sample.Playback
}

// AddSample creates a new Sample entity in the given world with the provided components.
func AddSample(w *ecs.World, s Sample) id.ID {
	sid := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", sid, s.ModuleID)

	ecs.SetComponent(w, auto, sid, s.Data)
	ecs.SetComponent(w, auto, sid, s.Flags)
	ecs.SetComponent(w, auto, sid, s.Infos)
	ecs.SetComponent(w, auto, sid, s.Volume)
	ecs.SetComponent(w, auto, sid, s.Playback)

	return sid
}

// GetSample retrieves a Sample entity from the world by its ID.
func GetSample(w *ecs.World, sid id.ID) (Sample, bool) {
	var s Sample

	if !ecs.GetComponent(w, "ModuleID", sid, &s.ModuleID) ||
		!ecs.GetComponent(w, auto, sid, &s.Data) ||
		!ecs.GetComponent(w, auto, sid, &s.Flags) ||
		!ecs.GetComponent(w, auto, sid, &s.Infos) ||
		!ecs.GetComponent(w, auto, sid, &s.Volume) ||
		!ecs.GetComponent(w, auto, sid, &s.Playback) {
		return s, false
	}

	return s, true
}

// DeleteSample removes a Sample entity from the world by its ID.
func DeleteSample(w *ecs.World, sid id.ID) {
	ecs.RemoveEntity(w, sid)
}
