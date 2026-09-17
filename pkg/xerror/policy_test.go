// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package xerror

import (
	"errors"
	"fmt"
	"testing"
)

func TestRegistryRegistration(t *testing.T) {
	registry := NewPolicy()

	global := registry.Global("global")

	exported := registry.Export("exported")

	if registry.global != global {
		t.Fatalf("Global() stored %v, want %v", registry.global, global)
	}

	if len(registry.exported) != 1 || registry.exported[0] != exported {
		t.Fatalf("Export() stored %v, want [%v]", registry.exported, exported)
	}
}

func TestRegistryBubbleDeduplicatesErrors(t *testing.T) {
	registry := NewPolicy()
	bubbled := errors.New("bubbled")

	registry.Bubble(bubbled)
	registry.Bubble(bubbled)

	if len(registry.bubbled) != 1 || registry.bubbled[0] != bubbled {
		t.Fatalf("Bubble() stored %v, want [%v]", registry.bubbled, bubbled)
	}
}

func TestRegistryMkError(t *testing.T) {
	base := errors.New("base")

	wrap := errors.New("wrap")

	alreadyWrapped := fmt.Errorf("context: %w", base)

	outer := fmt.Errorf("outer: %w", alreadyWrapped)

	bubbledContext := fmt.Errorf("context: %w", base)

	tests := []struct {
		name    string
		setup   func(*Policy)
		wrap    error
		context any
		want    error
		wantErr string
		same    bool
	}{
		{
			name:    "nil context",
			wrap:    wrap,
			context: nil,
			want:    nil,
		},
		{
			name:    "wraps plain context",
			wrap:    wrap,
			context: "context",
			want:    wrap,
			wantErr: "wrap: context",
		},
		{
			name:    "preserves already wrapped error",
			wrap:    outer,
			context: alreadyWrapped,
			want:    base,
			same:    true,
		},
		{
			name: "passes through bubbled error",
			setup: func(registry *Policy) {
				registry.Bubble(base)
			},
			wrap:    wrap,
			context: bubbledContext,
			want:    base,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registry := NewPolicy()
			if test.setup != nil {
				test.setup(registry)
			}

			result := registry.MkError(test.wrap, test.context)
			if test.want == nil {
				if result != nil {
					t.Fatalf("MkError() = %v, want nil", result)
				}
				return
			}
			if result == nil || !errors.Is(result, test.want) {
				t.Fatalf("MkError() = %v, want to contain %v", result, test.want)
			}
			if test.same && result != test.wrap {
				t.Fatalf("MkError() = %v, want the original wrapper", result)
			}
			if test.wantErr != "" && result.Error() != test.wantErr {
				t.Fatalf("MkError().Error() = %q, want %q", result, test.wantErr)
			}
		})
	}
}

func TestRegistryMkErrorJoinsGlobalWithExportedError(t *testing.T) {
	registry := NewPolicy()
	global := registry.Global("global")
	exported := registry.Export("exported")
	wrappedExported := fmt.Errorf("%w", exported)

	result := registry.MkError(wrappedExported, "context")

	if !errors.Is(result, global) || !errors.Is(result, exported) {
		t.Fatalf("MkError() = %v, want global and exported errors", result)
	}

	want := "global: exported: context"

	if result.Error() != want {
		t.Fatalf("MkError().Error() = %q, want %q", result, want)
	}
}

func TestRegistryMkErrorUnwrapsGlobalContext(t *testing.T) {
	registry := NewPolicy()

	_ = registry.Global("global")

	context := fmt.Errorf("%w: detail", registry.global)

	result := registry.MkError(errors.New("wrap"), context)

	want := "wrap: detail"

	if result.Error() != want {
		t.Fatalf("MkError() = %v, want %s", result, want)
	}
}

func TestRegistryMkErrorOnlyOneTopGlobalError(t *testing.T) {
	registry := NewPolicy()

	global := registry.Global("global")
	random0 := registry.Export("random0 error")

	random1 := errors.New("random1 error")

	alreadyGlobal := errors.Join(global, random1)

	result := registry.MkError(random0, alreadyGlobal)

	if !errors.Is(result, global) {
		t.Fatalf("MkError() = %v, want global error", result)
	}

	want := "global: random0 error: random1 error"

	if result.Error() != want {
		t.Fatalf("MkError().Error() = %q, want %q", result, want)
	}
}

func TestRegistryMkErrorRemovesInternalGlobalError(t *testing.T) {
	registry := NewPolicy()

	global := registry.Global("global")

	random0 := errors.New("random0 error")
	random1 := errors.New("random1 error")

	alreadyGlobal := errors.Join(global, random1)

	result := registry.MkError(random0, alreadyGlobal)

	if errors.Is(result, global) {
		t.Fatalf("MkError() = %v, should not be global", result)
	}

	want := "random0 error: random1 error"

	if result.Error() != want {
		t.Fatalf("MkError().Error() = %q, want %q", result, want)
	}
}
