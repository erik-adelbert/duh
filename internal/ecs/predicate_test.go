// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ecs

import "testing"

func TestNewPredicate_WithExplicitName(t *testing.T) {
	customName := "custom_component"

	p := NewPredicate(customName, func(v int) bool {
		return v > 0
	})

	if p.Target() != customName {
		t.Errorf("expected target %q, got %q", customName, p.Target())
	}
}

func TestNewPredicate_WithAutoName(t *testing.T) {
	p := NewPredicate(AutoName, func(v string) bool {
		return len(v) > 0
	})

	expected := NameFor[string]()
	if p.Target() != expected {
		t.Errorf("expected auto-generated name %q, got %q", expected, p.Target())
	}
}

func TestPredicate_Test_ReturnsTrue(t *testing.T) {
	p := NewPredicate("test", func(v int) bool {
		return v > 5
	})

	if !p.Test(10) {
		t.Error("expected Test(10) to return true")
	}
}

func TestPredicate_Test_ReturnsFalse(t *testing.T) {
	p := NewPredicate("test", func(v int) bool {
		return v > 5
	})

	if p.Test(3) {
		t.Error("expected Test(3) to return false")
	}
}

func TestPredicate_Test_WithStringType(t *testing.T) {
	p := NewPredicate("strings", func(v string) bool {
		return len(v) > 5
	})

	if !p.Test("hello world") {
		t.Error("expected Test('hello world') to return true")
	}

	if p.Test("hi") {
		t.Error("expected Test('hi') to return false")
	}
}

func TestPredicate_Test_WithComplexType(t *testing.T) {
	type Custom struct {
		Value int
		Name  string
	}

	p := NewPredicate("custom", func(v Custom) bool {
		return v.Value > 0 && len(v.Name) > 0
	})

	if !p.Test(Custom{Value: 42, Name: "test"}) {
		t.Error("expected Test with valid Custom to return true")
	}

	if p.Test(Custom{Value: -1, Name: "test"}) {
		t.Error("expected Test with negative Value to return false")
	}

	if p.Test(Custom{Value: 42, Name: ""}) {
		t.Error("expected Test with empty Name to return false")
	}
}

func TestPredicate_Target_ReturnsCorrectValue(t *testing.T) {
	tests := []string{"comp1", "component_2", "my-component"}

	for _, name := range tests {
		p := NewPredicate(name, func(v int) bool { return true })

		if p.Target() != name {
			t.Errorf("expected Target() to return %q, got %q", name, p.Target())
		}
	}
}

func BenchmarkNewPredicate(b *testing.B) {
	for b.Loop() {
		NewPredicate("bench", func(v int) bool {
			return v > 0
		})
	}
}

func BenchmarkPredicate_Test(b *testing.B) {
	p := NewPredicate("bench", func(v int) bool {
		return v > 50
	})

	b.ResetTimer()

	i := 0
	for b.Loop() {
		p.Test(i)
		i++
	}
}

func BenchmarkPredicate_Target(b *testing.B) {
	p := NewPredicate("bench", func(v int) bool {
		return true
	})

	b.ResetTimer()

	for b.Loop() {
		_ = p.Target()
	}
}
