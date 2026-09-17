// Copyright (c) 2024 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xerror provides a package level error classification and
// propagation policy.
//
// It classifies errors as global, exported, or bubbled, and provides
// consistent contextual wrapping while preserving Go's error chains.
// The policy also prevents redundant wrapping and separates internal
// errors from errors exposed through the public API.
//
// The MkError function is the core of this policy, ensuring that errors
// are handled consistently throughout the user package according to the
// defined rules:
//   - nil context results in a nil error (no error)
//   - bubbled errors pass through unchanged
//   - there is one global package error that is not double-wrapped
//   - exported errors get the global (error) classification
//   - context gets attached
//   - existing error chains are preserved where possible
//   - internal errors are not exposed to the user.
package xerror

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type Policy struct {
	global   error
	bubbled  []error
	exported []error
}

func NewPolicy() *Policy {
	return &Policy{
		bubbled:  []error{},
		exported: []error{},
	}
}

func (p *Policy) Global(text string) (err error) {
	err = errors.New(text)
	p.global = err

	return
}

func (p *Policy) Export(text string) (err error) {
	err = errors.New(text)
	p.exported = append(p.exported, err)

	return
}

func (p *Policy) Reexport(err error) error {
	p.exported = append(p.exported, err)
	return err
}

func (p *Policy) Bubble(err error) {
	if !slices.Contains(p.bubbled, err) {
		p.bubbled = append(p.bubbled, err)
	}
}

func (p *Policy) MkErrorf(wrap error, format string, a ...any) error {
	context := fmt.Sprintf(format, a...)

	return p.MkError(wrap, context)
}

func (p *Policy) MkError(wrap error, context any) error {
	if context == nil {
		// no error, pass through
		return nil
	}

	// If the context is an error, process it.
	ctxErr, ok := context.(error)

	if ok {
		// If the context error is one of the bubbled errors pass it through.
		for _, e := range p.bubbled {
			if errors.Is(ctxErr, e) {
				return ctxErr
			}
		}

		// If the context error is the global error, unwrap it to avoid double wrapping.
		if errors.Is(ctxErr, p.global) {
			switch u := ctxErr.(type) {
			case joined:
				errs := u.Unwrap()

				unwrapped := errs[:0]

				for _, e := range errs {
					if !errors.Is(e, p.global) {
						unwrapped = append(unwrapped, e)
					}
				}

				context = errors.Join(unwrapped...)
			case wrapped:
				err := u.Unwrap()

				context = err

				if errors.Is(err, p.global) {
					context = strings.TrimPrefix(ctxErr.Error(), p.global.Error()+": ")
				}
			}
		}
	}

	// Context is sane.

	// Wrap the flattened context with the provided error if it's not already wrapped.
	if !ok || !errors.Is(wrap, ctxErr) {
		wrap = fmt.Errorf("%w: %v", wrap, context)
	}

	// If the error is one of the exported errors, wrap it with global
	if p.global != nil && !errors.Is(wrap, p.global) {
		for _, e := range p.exported {
			if errors.Is(wrap, e) {
				return fmt.Errorf("%w: %w", p.global, wrap)
			}
		}
	}

	return wrap
}

type joined interface {
	Unwrap() []error
}

type wrapped interface {
	Unwrap() error
}
