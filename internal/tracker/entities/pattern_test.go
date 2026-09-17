package entities

import (
	"reflect"
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/help"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/pattern"
)

func testPattern() Pattern {
	data := help.NewMatrix[pattern.Cell](2, 3)
	data.Set(0, 0, pattern.Cell{Note: 60, Instru: 1, Volume: 32, Effect: 4, Param: 12})
	data.Set(0, 1, pattern.Cell{Note: 61, Instru: 2, Volume: 33, Effect: 5, Param: 13})
	data.Set(1, 2, pattern.Cell{Note: 72, Instru: 3, Volume: 48, Effect: 7, Param: 64})

	return Pattern{
		ModuleID: 42,
		Data:     data,
		Metadata: pattern.Metadata{
			Name:    "verse-a",
			Comment: "test pattern",
		},
	}
}

func TestAddPattern_GetPattern_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	want := testPattern()
	pid := AddPattern(w, want)

	got, ok := GetPattern(w, pid)

	if !ok {
		t.Fatal("expected pattern to be found")
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pattern mismatch: got %#v, want %#v", got, want)
	}
}

func TestGetPattern_MissingComponent_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	eid := ecs.NewEntity(w)
	ecs.SetComponent(w, "ModuleID", eid, id.ID(99))

	_, ok := GetPattern(w, eid)

	if ok {
		t.Fatal("expected GetPattern to fail when required components are missing")
	}
}

func TestDeletePattern_RemovesComponents(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	pid := AddPattern(w, testPattern())

	DeletePattern(w, pid)

	_, ok := GetPattern(w, pid)

	if ok {
		t.Fatal("expected deleted pattern to be unavailable")
	}
}

func BenchmarkAddPattern(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	pattern := testPattern()

	b.ResetTimer()

	for b.Loop() {
		AddPattern(w, pattern)
	}
}

func BenchmarkGetPattern(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	pid := AddPattern(w, testPattern())

	b.ResetTimer()

	for b.Loop() {
		GetPattern(w, pid)
	}
}

func BenchmarkDeletePattern(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	pattern := testPattern()

	b.ResetTimer()

	for b.Loop() {
		pid := AddPattern(w, pattern)
		DeletePattern(w, pid)
	}
}
