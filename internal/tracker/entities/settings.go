package entities

import (
	"github.com/erik-adelbert/duh/internal/audio"
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
)

type Settings = audio.Config

func AddSettings(w *ecs.World, s Settings) id.ID {
	sid := ecs.NewEntity(w)

	ecs.SetComponent(w, auto, sid, s)

	return sid
}

func GetSettings(w *ecs.World, sid id.ID) (Settings, bool) {
	return ecs.ComponentAs[Settings](w, auto, sid)
}

func DeleteSettings(w *ecs.World, sid id.ID) {
	ecs.RemoveEntity(w, sid)
}
