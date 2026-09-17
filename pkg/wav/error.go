// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package wav

import (
	"errors"
	"io"

	"github.com/erik-adelbert/duh/pkg/pcm"
	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

// ErrWAV is returned as a global wav error.
var ErrWAV = errp.Global("wav error")

// Bubbled errors are errors that are passed through unchanged.
func init() {
	errp.Bubble(io.EOF)
	errp.Bubble(io.ErrShortWrite)
}

// Exported errors for the wav package
var (
	ErrFormat = errp.Reexport(pcm.ErrFormat)

	// ErrDecode is returned when a read operation fails.
	ErrDecode = errp.Export("decode error")

	// ErrEncode is returned when a write operation fails.
	ErrEncode = errp.Export("encode error")

	// ErrSeek is returned when a seek operation fails.
	ErrSeek = errp.Export("seek error")

	// ErrESPIPE is returned when an illegal seek is attempted.
	ErrESPIPE = errp.Export("illegal seek")
)

var errFMT = errors.New("format error")

var mkError = errp.MkError
