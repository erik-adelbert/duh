package entities_test

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/audio"
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/entities"
)

func newTestSettings() entities.Settings {
	return audio.DefaultConfig()
}

func TestAddSettings_ReturnsValidID(t *testing.T) {
	w := ecs.NewWorld()
	s := newTestSettings()

	sid := entities.AddSettings(w, s)

	if sid == id.Null() {
		t.Error("expected valid id, got null")
	}
}

func TestAddSettings_MultipleReturnDistinctIDs(t *testing.T) {
	w := ecs.NewWorld()

	sid1 := entities.AddSettings(w, newTestSettings())
	sid2 := entities.AddSettings(w, newTestSettings())

	if sid1 == sid2 {
		t.Error("expected distinct ids for distinct settings")
	}
}

func TestGetSettings_AfterAdd_ReturnsTrue(t *testing.T) {
	w := ecs.NewWorld()

	sid := entities.AddSettings(w, newTestSettings())
	_, ok := entities.GetSettings(w, sid)

	if !ok {
		t.Error("expected GetSettings to return true after AddSettings")
	}
}

func TestGetSettings_AfterAdd_PreservesFields(t *testing.T) {
	w := ecs.NewWorld()
	s := audio.Config{
		Features:          audio.EnableReverb | audio.EnableXBass,
		Channels:          2,
		BitDepth:          24,
		SampleRate:        48_000,
		ResamplingMode:    audio.ResampleSpline,
		StereoSeparation:  0.75,
		MaxMixingChannels: 64,
		Looping:           1,
	}

	sid := entities.AddSettings(w, s)
	got, ok := entities.GetSettings(w, sid)

	if !ok {
		t.Fatal("expected GetSettings to succeed")
	}
	if got != s {
		t.Errorf("expected %+v, got %+v", s, got)
	}
}

func TestGetSettings_DefaultConfig_PreservesFields(t *testing.T) {
	w := ecs.NewWorld()
	s := audio.DefaultConfig()

	sid := entities.AddSettings(w, s)
	got, ok := entities.GetSettings(w, sid)

	if !ok {
		t.Fatal("expected GetSettings to succeed")
	}
	if got != s {
		t.Errorf("expected default config %+v, got %+v", s, got)
	}
}

func TestGetSettings_UnknownID_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	unknown := id.ID(999)

	_, ok := entities.GetSettings(w, unknown)

	if ok {
		t.Error("expected GetSettings to return false for unknown id")
	}
}

func TestDeleteSettings_RemovesEntity(t *testing.T) {
	w := ecs.NewWorld()

	sid := entities.AddSettings(w, newTestSettings())
	entities.DeleteSettings(w, sid)

	_, ok := entities.GetSettings(w, sid)
	if ok {
		t.Error("expected GetSettings to return false after DeleteSettings")
	}
}

func TestDeleteSettings_DoesNotAffectOthers(t *testing.T) {
	w := ecs.NewWorld()

	sid1 := entities.AddSettings(w, newTestSettings())
	sid2 := entities.AddSettings(w, newTestSettings())

	entities.DeleteSettings(w, sid1)

	_, ok := entities.GetSettings(w, sid2)
	if !ok {
		t.Error("expected sibling settings to remain after deleting another")
	}
}

func TestDeleteSettings_CalledTwice_DoesNotPanic(t *testing.T) {
	w := ecs.NewWorld()
	sid := entities.AddSettings(w, newTestSettings())

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic on double delete: %v", r)
		}
	}()

	entities.DeleteSettings(w, sid)
	entities.DeleteSettings(w, sid)
}

func BenchmarkAddSettings(b *testing.B) {
	w := ecs.NewWorld()
	s := newTestSettings()

	b.ResetTimer()
	for b.Loop() {
		entities.AddSettings(w, s)
	}
}

func BenchmarkGetSettings(b *testing.B) {
	w := ecs.NewWorld()
	sid := entities.AddSettings(w, newTestSettings())

	b.ResetTimer()
	for b.Loop() {
		entities.GetSettings(w, sid)
	}
}

func BenchmarkDeleteSettings(b *testing.B) {
	w := ecs.NewWorld()

	sids := make([]id.ID, 64)
	for i := range sids {
		sids[i] = entities.AddSettings(w, newTestSettings())
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		entities.DeleteSettings(w, sids[k])
		k++

		if k >= len(sids) {
			b.StopTimer()

			k = 0
			for i := range sids {
				sids[i] = entities.AddSettings(w, newTestSettings())
			}

			b.StartTimer()
		}
	}
}

func BenchmarkAddDeleteSettings(b *testing.B) {
	w := ecs.NewWorld()
	s := newTestSettings()

	b.ResetTimer()
	for b.Loop() {
		sid := entities.AddSettings(w, s)
		entities.DeleteSettings(w, sid)
	}
}
