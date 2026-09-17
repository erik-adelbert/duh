// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pp20

import (
	"errors"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

// ErrPP20 is returned as a global pp20 error.
var ErrPP20 = errp.Global("pp20 error")

// Exported errors for the pp20 package
var (
	// ErrArchive is returned when the archive is invalid or corrupted.
	ErrArchive = errp.Export("archive error")

	// ErrSeek is returned when a seek operation fails.
	ErrSeek = errp.Export("seek error")
)

// These internal errors serve as context for mkError.
var (
	errBits     = errors.New("bitstream error")
	errDecrunch = errors.New("decompression error")
	errFormat   = errors.New("not a valid file")
	errUnpack   = errors.New("unpack error")
)

var mkError = errp.MkError
