// duh midi driver for OPL3
//
// Based on:
//   Wohlstand libADLMIDI
//   bisqwit adlmidi
//
// Uses:
//   sneakernets DMXOPL wopl definitions
//
// Original:
// Copyright (c) 2010-2014 Joel Yliluoma
// Copyright (c) 2015-2026 Vitaly Novichkov
//
// Banks:
// Copyright (c) 2017 Shannon Freeman
//
// Go implementation and modifications:
// Copyright (C) 2026 Erik Adelbert
//
// This software is licensed under the GNU Lesser General Public
// License, version 2.1 or later.
//
// See LICENSE for the complete license text.

package mid

import (
	"errors"
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

func init() {
	errp.Bubble(io.EOF)
	errp.Bubble(io.ErrUnexpectedEOF)
}

var (
	ErrMidi    = errp.Global("midi error")
	ErrNotRIFF = errors.New("not a RIFF file")
)

var (
	ErrDB     = errp.Export("database error")
	ErrDecode = errp.Export("decode error")
	ErrSeek   = errp.Export("seek error")
)

var ErrBuiltin error

var mkError = errp.MkError
