package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	plugin "github.com/erik-adelbert/duh/internal/tracker/components/mixplugin"
)

type MixPlugin struct {
	ModuleID id.ID

	plugin.State
	plugin.Infos
	plugin.Mixer
	plugin.Plugin
}

func AddMixPlugin(w *ecs.World, x MixPlugin) id.ID {
	xid := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", xid, x.ModuleID)

	ecs.SetComponent(w, auto, xid, x.Mixer)
	ecs.SetComponent(w, auto, xid, x.State)
	ecs.SetComponent(w, auto, xid, x.Infos)
	ecs.SetComponent(w, auto, xid, x.Plugin)

	return xid
}

func GetMixPlugin(w *ecs.World, xid id.ID) (MixPlugin, bool) {
	var x MixPlugin

	if !ecs.GetComponent(w, "ModuleID", xid, &x.ModuleID) ||
		!ecs.GetComponent(w, auto, xid, &x.Mixer) ||
		!ecs.GetComponent(w, auto, xid, &x.State) ||
		!ecs.GetComponent(w, auto, xid, &x.Infos) ||
		!ecs.GetComponent(w, auto, xid, &x.Plugin) {
		return x, false
	}

	return x, true
}

func DeleteMixPlugin(w *ecs.World, xid id.ID) {
	ecs.RemoveEntity(w, xid)
}
