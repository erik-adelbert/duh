package entities

import (
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/order"
)

func testOrder() Order {
	return Order{
		ModuleID:     42,
		PatternIndex: order.PatternIndex(7),
	}
}

func TestAddOrder_GetOrder_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	want := testOrder()
	oid := AddOrder(w, want)

	got, ok := GetOrder(w, oid)

	if !ok {
		t.Fatal("expected order to be found")
	}

	if got != want {
		t.Fatalf("order mismatch: got %#v, want %#v", got, want)
	}
}

func TestGetOrder_MissingComponent_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	eid := ecs.NewEntity(w)
	ecs.SetComponent(w, "ModuleID", eid, id.ID(99))

	_, ok := GetOrder(w, eid)

	if ok {
		t.Fatal("expected GetOrder to fail when required components are missing")
	}
}

func TestDeleteOrder_RemovesComponents(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	oid := AddOrder(w, testOrder())

	DeleteOrder(w, oid)

	_, ok := GetOrder(w, oid)

	if ok {
		t.Fatal("expected deleted order to be unavailable")
	}
}

func BenchmarkAddOrder(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	b.ResetTimer()

	for b.Loop() {
		AddOrder(w, testOrder())
	}
}

func BenchmarkGetOrder(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	oid := AddOrder(w, testOrder())

	b.ResetTimer()

	for b.Loop() {
		GetOrder(w, oid)
	}
}

func BenchmarkDeleteOrder(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	b.ResetTimer()

	for b.Loop() {
		oid := AddOrder(w, testOrder())
		DeleteOrder(w, oid)
	}
}
