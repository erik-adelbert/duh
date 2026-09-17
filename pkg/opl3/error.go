// duh OPL3 emulator package
//
// Based on Nuked OPL3 by Nuke.YKT.
//
// Original:
// Copyright (C) 2026 Tony Gies (Nuked-OPL3-fast modifications)
// Copyright (C) 2013-2020 Nuke.YKT
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package opl3

import (
	"errors"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

// ErrOPL3 is returned as a global opl3 error.
var ErrOPL3 = errp.Global("opl3 error")

// Exported errors for the opl3 package
var (
	ErrDevice = errp.Export("device error")
	ErrVGM    = errp.Export("vgm error")
)

// These internal errors serve as context for mkError
var (
	errWriteAddress = errors.New("write address error")
	errWriteData    = errors.New("write data error")
	errSampleRate   = errors.New("sample rate error")
)

var mkError = errp.MkError
