// Functions and types for Channel entities (audio playback channels).
package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	vo "github.com/erik-adelbert/duh/internal/tracker/components/voice"
)

// Voice holds all components for an audio channel.
// It is a convenience struct for creating entities with all relevant components.
type Voice struct {
	ModuleID id.ID

	vo.Automation
	vo.Instrument
	vo.Defaults
	vo.Envelope
	vo.VUMeter
	vo.MixerRoutine
	vo.MixingControl
	vo.MixingParams
	vo.Balance
	vo.Flags
	vo.Swing
}

// AddVoice creates a new audio channel entity in the given world with the provided components.
func AddVoice(w *ecs.World, v Voice) id.ID {
	voID := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", voID, v.ModuleID)

	ecs.SetComponent(w, auto, voID, v.Automation)
	ecs.SetComponent(w, auto, voID, v.Instrument)
	ecs.SetComponent(w, auto, voID, v.Defaults)
	ecs.SetComponent(w, auto, voID, v.Envelope)
	ecs.SetComponent(w, auto, voID, v.VUMeter)
	ecs.SetComponent(w, auto, voID, v.MixerRoutine)
	ecs.SetComponent(w, auto, voID, v.MixingControl)
	ecs.SetComponent(w, auto, voID, v.MixingParams)
	ecs.SetComponent(w, auto, voID, v.Balance)
	ecs.SetComponent(w, auto, voID, v.Flags)
	ecs.SetComponent(w, auto, voID, v.Swing)

	return voID
}

// GetVoice retrieves an audio channel entity from the world by its ID.
func GetVoice(w *ecs.World, voID id.ID) (Voice, bool) {
	var v Voice

	if !ecs.GetComponent(w, "ModuleID", voID, &v.ModuleID) ||
		!ecs.GetComponent(w, auto, voID, &v.Automation) ||
		!ecs.GetComponent(w, auto, voID, &v.Instrument) ||
		!ecs.GetComponent(w, auto, voID, &v.Defaults) ||
		!ecs.GetComponent(w, auto, voID, &v.Envelope) ||
		!ecs.GetComponent(w, auto, voID, &v.VUMeter) ||
		!ecs.GetComponent(w, auto, voID, &v.MixerRoutine) ||
		!ecs.GetComponent(w, auto, voID, &v.MixingControl) ||
		!ecs.GetComponent(w, auto, voID, &v.MixingParams) ||
		!ecs.GetComponent(w, auto, voID, &v.Balance) ||
		!ecs.GetComponent(w, auto, voID, &v.Flags) ||
		!ecs.GetComponent(w, auto, voID, &v.Swing) {
		return v, false
	}

	return v, true
}

// DeleteVoice removes an audio channel entity from the world by its ID.
func DeleteVoice(w *ecs.World, eid id.ID) {
	ecs.RemoveEntity(w, eid)
}
