package entities_test

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/sample"
	"github.com/erik-adelbert/duh/internal/tracker/entities"
)

func newTestWorld(t *testing.T) *ecs.World {
	t.Helper()
	w := ecs.NewWorld()
	if w == nil {
		t.Fatal("failed to create world")
	}
	return w
}

func newTestSample(moduleID id.ID) entities.Sample {
	return entities.Sample{
		ModuleID: moduleID,
		Data:     sample.Data{},
		Flags:    0,
		Infos:    sample.Infos{},
		Volume:   sample.Volume{},
		Playback: sample.Playback{},
	}
}

func TestAddSample_ReturnsValidID(t *testing.T) {
	w := newTestWorld(t)
	s := newTestSample(123)

	sid := entities.AddSample(w, s)

	if sid == id.Null() {
		t.Error("expected valid id, got zero")
	}
}

func TestAddSample_MultipleReturnDistinctIDs(t *testing.T) {
	w := newTestWorld(t)

	sid1 := entities.AddSample(w, newTestSample(123))
	sid2 := entities.AddSample(w, newTestSample(456))

	if sid1 == sid2 {
		t.Error("expected distinct ids for distinct samples")
	}
}

func TestGetSample_AfterAdd_ReturnsTrue(t *testing.T) {
	w := newTestWorld(t)
	s := newTestSample(123)

	sid := entities.AddSample(w, s)
	_, ok := entities.GetSample(w, sid)

	if !ok {
		t.Error("expected GetSample to return true after AddSample")
	}
}

func TestGetSample_AfterAdd_PreservesModuleID(t *testing.T) {
	w := newTestWorld(t)
	moduleID := id.ID(123)
	s := newTestSample(moduleID)

	sid := entities.AddSample(w, s)
	got, ok := entities.GetSample(w, sid)

	if !ok {
		t.Fatal("expected GetSample to succeed")
	}
	if got.ModuleID != moduleID {
		t.Errorf("expected ModuleID %v, got %v", moduleID, got.ModuleID)
	}
}

func TestGetSample_UnknownID_ReturnsFalse(t *testing.T) {
	w := newTestWorld(t)
	unknown := id.ID(123)

	_, ok := entities.GetSample(w, unknown)

	if ok {
		t.Error("expected GetSample to return false for unknown id")
	}
}

func TestDeleteSample_RemovesEntity(t *testing.T) {
	w := newTestWorld(t)
	s := newTestSample(123)

	sid := entities.AddSample(w, s)
	entities.DeleteSample(w, sid)

	_, ok := entities.GetSample(w, sid)
	if ok {
		t.Error("expected GetSample to return false after DeleteSample")
	}
}

func TestDeleteSample_DoesNotAffectOthers(t *testing.T) {
	w := newTestWorld(t)

	sid1 := entities.AddSample(w, newTestSample(123))
	sid2 := entities.AddSample(w, newTestSample(456))

	entities.DeleteSample(w, sid1)

	_, ok := entities.GetSample(w, sid2)
	if !ok {
		t.Error("expected sibling sample to remain after deleting another")
	}
}

func TestDeleteSample_CalledTwice_DoesNotPanic(t *testing.T) {
	w := newTestWorld(t)
	sid := entities.AddSample(w, newTestSample(123))

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic on double delete: %v", r)
		}
	}()

	entities.DeleteSample(w, sid)
	entities.DeleteSample(w, sid)
}

func BenchmarkAddSample(b *testing.B) {
	w := ecs.NewWorld()
	s := newTestSample(id.ID(123))

	b.ResetTimer()

	for b.Loop() {
		entities.AddSample(w, s)
	}
}

func BenchmarkGetSample(b *testing.B) {
	w := ecs.NewWorld()
	sid := entities.AddSample(w, newTestSample(id.ID(123)))

	b.ResetTimer()
	for b.Loop() {
		entities.GetSample(w, sid)
	}
}

func BenchmarkDeleteSample(b *testing.B) {
	w := ecs.NewWorld()

	sids := make([]id.ID, 64)
	for i := range sids {
		sids[i] = entities.AddSample(w, newTestSample(id.ID(i)))
	}

	b.ResetTimer()

	k := 0
	for b.Loop() {
		entities.DeleteSample(w, sids[k])
		k++

		if k >= len(sids) {
			b.StopTimer()

			k = 0
			for i := range sids {
				sids[i] = entities.AddSample(w, newTestSample(id.ID(i)))
			}

			b.StartTimer()
		}
	}
}

func BenchmarkAddDeleteSample(b *testing.B) {
	w := ecs.NewWorld()
	s := newTestSample(id.ID(123))

	for b.Loop() {
		sid := entities.AddSample(w, s)
		entities.DeleteSample(w, sid)
	}
}
