package mod

import (
	"errors"
	"fmt"
)

var ErrMod = newGlobalError("MOD error")

var (
	// ErrNew is returned when the input is not a valid MOD.
	ErrNew = newExportedError("invalid source")
)

var (
	errSignature = errors.New("unknown signature")
	errRead      = errors.New("read error")
)

// mkError creates a new error with context for the given error.
// A context is either a string or an error. If the context is nil, mkError returns nil.
// If the error is exported, it wraps the context as well
// and emits the global error.
//
// Exported functions should call mkError with an exported (title-cased) error.
func mkError(err error, context any) error {
	// If the context is nil, return nil to avoid wrapping errors with no context.
	if context == nil {
		return nil
	}

	err = fmt.Errorf("%w: %v", err, context)

	// If the error is one of the exported errors, wrap it with global
	if global != nil {
		for _, e := range exported {
			if errors.Is(err, e) {
				return errors.Join(global, err)
			}
		}
	}

	return err
}

var (
	// Exported errors internal registry for mkError to check against.
	exported = []error{}

	// Global error for mkError to wrap exported errors with.
	global error
)

// newGlobalError creates a new global error and registers it.
func newGlobalError(text string) (err error) {
	err = errors.New(text)
	global = err

	return
}

// newExportedError creates a new exported error and registers it.
func newExportedError(text string) (err error) {
	err = errors.New(text)
	exported = append(exported, err)

	return
}
