// Functions and types for OrderList entities (pattern sequence).
package entities

import (
	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/order"
)

type Order struct {
	ModuleID id.ID

	order.PatternIndex
}

func AddOrder(w *ecs.World, o Order) id.ID {
	oid := ecs.NewEntity(w)

	ecs.SetComponent(w, "ModuleID", oid, o.ModuleID)
	ecs.SetComponent(w, auto, oid, o.PatternIndex)

	return oid
}

func GetOrder(w *ecs.World, oid id.ID) (Order, bool) {
	var o Order

	if !ecs.GetComponent(w, "ModuleID", oid, &o.ModuleID) ||
		!ecs.GetComponent(w, auto, oid, &o.PatternIndex) {
		return o, false
	}

	return o, true
}

func DeleteOrder(w *ecs.World, oid id.ID) {
	ecs.RemoveEntity(w, oid)
}
