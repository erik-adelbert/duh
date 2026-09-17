// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package entities

import (
	"reflect"
	"testing"

	"github.com/erik-adelbert/duh/internal/ecs"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
	env "github.com/erik-adelbert/duh/internal/tracker/components/envelope"
	ins "github.com/erik-adelbert/duh/internal/tracker/components/instrument"
)

func testInstrument() Instrument {
	return Instrument{
		ModuleID: 42,
		Balance:  core.Balance[uint16]{Vol: 3210, Pan: 40},
		Envelope: env.Envelope{
			Flags: env.EnvVolume | env.EnvPanning,
		},
		Filter: ins.Filter{CutOff: 99, Resonance: 55},
		Metadata: ins.Metadata{
			Name:     "Lead",
			Filename: "lead.it",
		},
		Midi:         ins.Midi{Bank: 10, Prog: 8, Channel: 1, DrumKey: 60},
		NoteBehavior: ins.NoteBehavior{NNA: 1, DCT: 2, DCA: 3},
		Mappings: ins.Mappings{
			Keys:  [128]uint8{0: 1, 1: 2, 2: 3},
			Notes: [128]uint8{0: 4, 1: 5, 2: 6},
		},
		PitchPan:       ins.PitchPan{Separation: 33, Center: 64},
		Params:         core.Voice[ins.Params]{},
		Swing:          core.Balance[uint8]{Vol: 15, Pan: 20},
		DefaultPanning: 90,
		GlobalVolume:   110,
		FadeOut:        7,
	}
}

func equals(a, b Instrument) bool {
	return a.ModuleID == b.ModuleID &&
		a.Balance == b.Balance &&
		a.Filter == b.Filter &&
		a.Metadata == b.Metadata &&
		a.Midi == b.Midi &&
		a.NoteBehavior == b.NoteBehavior &&
		a.Mappings == b.Mappings &&
		a.PitchPan == b.PitchPan &&
		a.Params == b.Params &&
		a.Swing == b.Swing &&
		a.DefaultPanning == b.DefaultPanning &&
		a.GlobalVolume == b.GlobalVolume &&
		a.FadeOut == b.FadeOut &&
		reflect.DeepEqual(a.Envelope, b.Envelope)
}

func TestAddInstrument_GetInstrument_RoundTrip(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	want := testInstrument()
	iid := AddInstrument(w, want)

	got, ok := GetInstrument(w, iid)

	if !ok {
		t.Fatal("expected instrument to be found")
	}

	if !equals(got, want) {
		t.Fatalf("instrument mismatch: got %#v, want %#v", got, want)
	}
}

func TestGetInstrument_MissingComponent_ReturnsFalse(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	eid := ecs.NewEntity(w)
	ecs.SetComponent(w, "ModuleID", eid, id.ID(99))

	_, ok := GetInstrument(w, eid)

	if ok {
		t.Fatal("expected GetInstrument to fail when required components are missing")
	}
}

func TestDeleteInstrument_RemovesComponents(t *testing.T) {
	w := ecs.NewWorld()
	defer w.Close()

	iid := AddInstrument(w, testInstrument())

	DeleteInstrument(w, iid)

	_, ok := GetInstrument(w, iid)

	if ok {
		t.Fatal("expected deleted instrument to be unavailable")
	}
}

func BenchmarkAddInstrument(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	b.ResetTimer()

	for b.Loop() {
		AddInstrument(w, testInstrument())
	}
}

func BenchmarkGetInstrument(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	iid := AddInstrument(w, testInstrument())

	b.ResetTimer()

	for b.Loop() {
		GetInstrument(w, iid)
	}
}

func BenchmarkDeleteInstrument(b *testing.B) {
	w := ecs.NewWorld()
	defer w.Close()

	b.ResetTimer()

	for b.Loop() {
		iid := AddInstrument(w, testInstrument())
		DeleteInstrument(w, iid)
	}
}
