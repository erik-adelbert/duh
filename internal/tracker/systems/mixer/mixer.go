package mixer

import (
	"github.com/erik-adelbert/duh/internal/audio"
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
	ac "github.com/erik-adelbert/duh/internal/tracker/components/voice"
)

const auto = ecs.AutoName

type (
	Fp284  = audio.Fp284
	Buffer = audio.Buffer
)

// const chunkSize = 4

func StereoMix(w *ecs.World) {
	mid := getModule(w)

	if mid == id.Null() {
		// No module found, there is no point in mixing
		return
	}

	infos := new(mod.Metadata)

	if !ecs.GetComponent(w, auto, mid, infos) || infos.Status != mod.StatusMixing {
		// No metadata found or module is not mixing, cannot proceed
		return
	}

	mixbus := new(mod.MixBus)

	if !ecs.GetComponent(w, auto, mid, mixbus) {
		// No audio buffer found, nothing to mix to, cannot proceed
		return
	}

	buf := mixbus.SoundBuf
	sampleCount := buf.Len()

	voices := mixbus.Voices

	settings := getSettings(w)
	flags := ac.Flags(settings.Features)

	// mixed := channels[:0] // Reuse the slice to avoid allocations

	var off ac.Offset
	voiceMixer(w, mixbus, &off, voices, flags, sampleCount)

	// for cid := range out {
	// 	mixed = append(mixed, cid)
	// }
	// channels = mixed

	// Update mixbus component
	mixbus.Voices = voices

	ecs.SetComponent(w, auto, mid, mixbus)
}

var activeModule = ecs.NewPredicate(
	auto,
	func(infos mod.Metadata) bool {
		return infos.Status == mod.StatusMixing
	},
)

func getModule(w *ecs.World) id.ID {
	return ecs.First(ecs.AllEntitiesMatching(w, activeModule))
}

func getSettings(w *ecs.World) audio.Config {
	name := ecs.NameFor[audio.Config]()

	sid := ecs.First(ecs.AllEntitiesWith(w, name))
	settings, _ := ecs.ComponentAs[audio.Config](w, name, sid)

	return settings
}
