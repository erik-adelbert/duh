// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mmcmp

import (
	"errors"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

// ErrMMCMP is returned as a global mmcmp error.
var ErrMMCMP = errp.Global("mmcmp error")

// Exported errors for the mmcmp package
var (
	// ErrArchive is returned when the archive is invalid or corrupted.
	ErrArchive = errp.Export("archive error")

	// ErrSeek is returned when a seek operation fails.
	ErrSeek = errp.Export("seek error")
)

// These internal errors serve as context for mkError
var (
	errBits       = errors.New("bitstream error")
	errBlockTable = errors.New("block table error")
	errBlock      = errors.New("block error")
	errDecrunch   = errors.New("decompression error")
	errFormat     = errors.New("not a valid file")
	errRead       = errors.New("read error")
	errUnpack     = errors.New("unpack error")
)

var mkError = errp.MkError
