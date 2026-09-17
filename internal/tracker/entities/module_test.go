package entities

import (
	"reflect"
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
)

func testModule() Module {
	return Module{
		Flags: mod.Oversample | mod.Reverb | mod.LinearSlides,
		Metadata: mod.Metadata{
			Status: mod.StatusMixing,
			Format: mod.FormatIT,
			Counts: mod.Counts{Voices: 32, Samples: 16, Instrus: 8},
			Name:   "test-module",
			Speed:  6,
			Tempo:  125,
		},
		Playback: mod.Playback{
			Order: 1,
			Row:   32,
			Tick:  3,
			Speed: 6,
			Tempo: 125,
		},
		MixBus: mod.MixBus{
			Voices:     []id.ID{11, 12, 13},
			AGC:        mod.AGCState{Gain: 1.25, Envelope: 0.75},
			BufferPos:  64,
			ChunkSize:  128,
			ReverbSend: 16,
		},
		Orders:      []id.ID{1, 2, 3},
		Patterns:    []id.ID{4, 5},
		Samples:     []id.ID{6, 7, 8},
		Instruments: []id.ID{9, 10},
		Channels:    []id.ID{14, 15},
		Plugins:     []id.ID{16, 17},
		OutBuffer:   99,
	}
}

func TestAddModule_GetModule_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	want := testModule()
	mid := AddModule(w, want)

	got, ok := GetModule(w, mid)

	if !ok {
		t.Fatal("expected module to be found")
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("module mismatch: got %#v, want %#v", got, want)
	}
}

func TestGetModule_MissingComponent_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	eid := ecs.NewEntity(w)
	ecs.SetComponent(w, auto, eid, mod.Flags(mod.Oversample))

	_, ok := GetModule(w, eid)

	if ok {
		t.Fatal("expected GetModule to fail when required components are missing")
	}
}

func TestDeleteModule_RemovesComponents(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	mid := AddModule(w, testModule())

	DeleteModule(w, mid)

	_, ok := GetModule(w, mid)

	if ok {
		t.Fatal("expected deleted module to be unavailable")
	}
}

func BenchmarkAddModule(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()

	b.ResetTimer()

	for b.Loop() {
		AddModule(w, module)
	}
}

func BenchmarkGetModule(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	mid := AddModule(w, testModule())

	b.ResetTimer()

	for b.Loop() {
		GetModule(w, mid)
	}
}

func BenchmarkDeleteModule(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()

	b.ResetTimer()

	for b.Loop() {
		mid := AddModule(w, module)
		DeleteModule(w, mid)
	}
}
