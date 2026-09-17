// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vgmo3

import (
	"errors"
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

func init() {
	errp.Bubble(io.EOF) // Bubble EOF errors to the caller without wrapping them
}

// ErrVGM is returned as a global error wrapper.
var ErrVGM = errp.Global("vgm error")

// Exported errors for the vgm package
var (
	ErrNew    = errp.Export("new error")
	ErrFile   = errp.Export("file error")
	ErrRead   = errp.Export("read error")
	ErrStream = errp.Export("stream error")
)

// These internal errors serve as context for mkError
var (
	errHeader = errors.New("header error")
	errUngzip = errors.New("ungzip error")
)

var mkError = errp.MkError
