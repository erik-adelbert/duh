package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
)

type ByteSlice []byte

type OutBuffer struct {
	ByteSlice
}

func AddOutBuffer(w *ecs.World, buf OutBuffer) id.ID {
	mid := ecs.First(ecs.AllEntitiesWith(w, ecs.NameFor[mod.Metadata]()))

	if mid == id.Null() {
		// No module found, there is no point in adding the out buffer
		return id.Null()
	}

	bid := ecs.NewEntity(w)

	ecs.SetComponent(w, auto, bid, buf)
	ecs.SetComponent(w, "OutBuffer", mid, bid)

	// Update module status
	// Silently ignore errors, as the module must exist
	infos, _ := ecs.ComponentAs[mod.Metadata](w, auto, mid)
	infos.Status = mod.StatusMixing

	ecs.SetComponent(w, auto, mid, infos)

	return bid
}

func GetOutBuffer(w *ecs.World, bid id.ID) (OutBuffer, bool) {
	return ecs.ComponentAs[OutBuffer](w, auto, bid)
}

func DeleteOutBuffer(w *ecs.World, bid id.ID) {
	ecs.RemoveEntity(w, bid)
}
