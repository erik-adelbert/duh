// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"errors"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

var ErrBackend = errp.Global("backend error")

var (
	ErrPlayer      = errp.Export("player error")
	ErrNoOptions   = errors.New("no audio options")
	ErrNoDuration  = errors.New("no duration available")
	ErrInvalidSeek = errors.New("invalid seek operation")
	ErrBadFormat   = errors.New("unsupported audio format")
	ErrESPIPE      = errors.New("illegal seek")
)

var mkError = errp.MkError
