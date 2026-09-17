package s3m

import (
	"errors"
	"fmt"
	"slices"
)

var ErrS3M = newGlobalError("S3M error")

var (
	// ErrNew is returned when the input is not a valid S3M.
	ErrNew = newExportedError("invalid source")
)

var (
	errRead = errors.New("read error")
	errSeek = errors.New("seek error")
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

	// If the error is one of the exported errors, wrap it with global...
	if global != nil { // ... if defined
		for _, e := range exported {
			if errors.Is(err, e) {
				return errors.Join(global, err)
			}
		}
	}

	return err
}

// joinError joins multiple errors into a single error.
// It returns nil if all errors are nil.
// If any error is exported, it wraps the context as well
// and emits the global error.
func joinError(errs ...error) (err error) {
	for _, e := range slices.Backward(errs) {
		if e == nil {
			continue
		}

		switch err {
		case nil:
			err = e
		default:
			err = mkError(e, err)
		}
	}

	return
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
