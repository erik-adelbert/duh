package entities

import (
	"reflect"
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	mod "github.com/erik-adelbert/duh/internal/tracker/components/module"
)

func testOutBuffer() OutBuffer {
	return OutBuffer{ByteSlice: ByteSlice{1, 2, 3, 4}}
}

func TestAddOutBuffer_GetOutBuffer_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()
	module.Status = mod.StatusPaused
	module.OutBuffer = id.Null()
	mid := AddModule(w, module)
	want := testOutBuffer()

	bid := AddOutBuffer(w, want)
	got, ok := GetOutBuffer(w, bid)

	if bid == id.Null() {
		t.Fatal("expected out buffer entity to be created")
	}

	if !ok {
		t.Fatal("expected out buffer to be found")
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("out buffer mismatch: got %#v, want %#v", got, want)
	}

	moduleAfter, ok := GetModule(w, mid)

	if !ok {
		t.Fatal("expected module to be found after adding out buffer")
	}

	if moduleAfter.OutBuffer != bid {
		t.Fatalf("expected module out buffer to be %d, got %d", bid, moduleAfter.OutBuffer)
	}

	if moduleAfter.Status != mod.StatusMixing {
		t.Fatalf("expected module status to be %v, got %v", mod.StatusMixing, moduleAfter.Status)
	}
}

func TestAddOutBuffer_WithoutModule_ReturnsNull(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	bid := AddOutBuffer(w, testOutBuffer())

	if bid != id.Null() {
		t.Fatalf("expected null out buffer ID, got %d", bid)
	}

	if eid := ecs.NewEntity(w); eid != 1 {
		t.Fatalf("expected no entity allocation when module is missing, next entity was %d", eid)
	}
}

func TestDeleteOutBuffer_RemovesBuffer(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	AddModule(w, testModule())
	bid := AddOutBuffer(w, testOutBuffer())

	DeleteOutBuffer(w, bid)

	_, ok := GetOutBuffer(w, bid)

	if ok {
		t.Fatal("expected deleted out buffer to be unavailable")
	}
}

func BenchmarkAddOutBuffer(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()
	module.OutBuffer = id.Null()
	AddModule(w, module)

	out := testOutBuffer()

	b.ResetTimer()

	for b.Loop() {
		AddOutBuffer(w, out)
	}
}

func BenchmarkGetOutBuffer(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()
	module.OutBuffer = id.Null()
	AddModule(w, module)
	bid := AddOutBuffer(w, testOutBuffer())

	b.ResetTimer()

	for b.Loop() {
		GetOutBuffer(w, bid)
	}
}

func BenchmarkDeleteOutBuffer(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	module := testModule()
	module.OutBuffer = id.Null()
	AddModule(w, module)

	out := testOutBuffer()

	b.ResetTimer()

	for b.Loop() {
		bid := AddOutBuffer(w, out)
		DeleteOutBuffer(w, bid)
	}
}
