// duh pcm package
//
// Based on the UCB release of Plan 9 pcmconv.
//
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package pcm

import (
	"errors"
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

func init() {
	errp.Bubble(io.EOF)
}

// ErrPCM is the global error for PCM-related errors.
var ErrPCM = errp.Global("pcm error")

var (
	// ErrFormat is returned when the PCM format is invalid.
	ErrFormat = errp.Export("invalid format")

	// ErrNew is returned when the PCM conversion fails.
	ErrNew = errp.Export("failed to create converter")

	ErrConvert = errp.Export("failed to convert")

	ErrRing = errp.Export("ring buffer error")

	ErrTap = errp.Export("tap error")

	ErrRatio = errp.Export("invalid ratio")

	ErrFlush = errp.Export("failed to flush")

	ErrSeek = errp.Export("failed to seek")

	ErrWrite = errp.Export("failed to write")

	ErrClose = errp.Export("failed to close")

	ErrRead = errp.Export("failed to read")

	ErrESPIPE = errp.Export("illegal seek")
)

var (
	errBadFormat    = errors.New("bad format")
	errInvalidField = errors.New("invalid field")
	errInvalidValue = errors.New("invalid value")
)

var (
	mkError  = errp.MkError
	mkErrorf = errp.MkErrorf
)
