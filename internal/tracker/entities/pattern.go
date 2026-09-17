// Functions and types for Pattern entities (pattern grid, rows, etc.).
package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/pattern"
)

type Pattern struct {
	ModuleID id.ID

	pattern.Data
	pattern.Metadata
}

func AddPattern(w *ecs.World, a Pattern) id.ID {
	pid := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", pid, a.ModuleID)

	ecs.SetComponent(w, auto, pid, a.Data)
	ecs.SetComponent(w, auto, pid, a.Metadata)

	return pid
}

func GetPattern(w *ecs.World, pid id.ID) (Pattern, bool) {
	var p Pattern

	if !ecs.GetComponent(w, "ModuleID", pid, &p.ModuleID) ||
		!ecs.GetComponent(w, auto, pid, &p.Data) ||
		!ecs.GetComponent(w, auto, pid, &p.Metadata) {
		return p, false
	}

	return p, true
}

func DeletePattern(w *ecs.World, pid id.ID) {
	ecs.RemoveEntity(w, pid)
}
