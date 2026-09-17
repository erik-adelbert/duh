package entities

import (
	"reflect"
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
	plugin "github.com/erik-adelbert/duh/internal/tracker/components/mixplugin"
)

type mockMixer struct{}

func (*mockMixer) AddRef() int { return 1 }

func (*mockMixer) Release() int { return 1 }

func (*mockMixer) Init(freq uint, reset bool) {}

func (*mockMixer) Save() {}

func (*mockMixer) Load() {}

func (*mockMixer) Mix(outLeft, outRight []float32) {}

func (*mockMixer) MidiSend(msg uint32) {}

func (*mockMixer) MidiCommand(channel, program, note, volume uint32) {}

func testMixPlugin() MixPlugin {
	return MixPlugin{
		ModuleID: 7,
		State: plugin.State{
			Flags:    plugin.PluginEnabled | plugin.PluginWetDryMix,
			VolDecay: core.Stereo[int32]{Left: 11, Right: 22},
			Out: core.Stereo[[]float32]{
				Left:  []float32{0.25, 0.5},
				Right: []float32{0.75, 1.0},
			},
			Buf: []int32{3, 4, 5},
		},
		Infos: plugin.Infos{
			ID1:     101,
			ID2:     202,
			Name:    "test-plugin",
			Library: "test.vst",
			Routing: struct {
				In, Out uint32
				Infos   [4]uint32
			}{
				In:    2,
				Out:   4,
				Infos: [4]uint32{1, 2, 3, 4},
			},
		},
		Mixer: &mockMixer{},
		Plugin: plugin.Plugin{
			Data: "opaque",
			Size: 64,
		},
	}
}

func TestAddMixPlugin_GetMixPlugin_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	want := testMixPlugin()
	xid := AddMixPlugin(w, want)

	got, ok := GetMixPlugin(w, xid)

	if !ok {
		t.Fatal("expected mix plugin to be found")
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mix plugin mismatch: got %#v, want %#v", got, want)
	}
}

func TestGetMixPlugin_MissingComponent_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	eid := ecs.NewEntity(w)
	ecs.SetComponent(w, "ModuleID", eid, id.ID(77))

	_, ok := GetMixPlugin(w, eid)

	if ok {
		t.Fatal("expected GetMixPlugin to fail when required components are missing")
	}
}

func TestDeleteMixPlugin_RemovesComponents(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	xid := AddMixPlugin(w, testMixPlugin())

	DeleteMixPlugin(w, xid)

	_, ok := GetMixPlugin(w, xid)

	if ok {
		t.Fatal("expected deleted mix plugin to be unavailable")
	}
}

func BenchmarkAddMixPlugin(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	mixPlugin := testMixPlugin()

	b.ResetTimer()

	for b.Loop() {
		AddMixPlugin(w, mixPlugin)
	}
}

func BenchmarkGetMixPlugin(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	xid := AddMixPlugin(w, testMixPlugin())

	b.ResetTimer()

	for b.Loop() {
		GetMixPlugin(w, xid)
	}
}

func BenchmarkDeleteMixPlugin(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	mixPlugin := testMixPlugin()

	b.ResetTimer()

	for b.Loop() {
		xid := AddMixPlugin(w, mixPlugin)
		DeleteMixPlugin(w, xid)
	}
}
