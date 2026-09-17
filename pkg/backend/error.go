// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import "errors"

var (
	ErrNoOptions   = errors.New("no audio options")
	ErrNoDuration  = errors.New("no duration available")
	ErrInvalidSeek = errors.New("invalid seek operation")
	ErrBadFormat   = errors.New("unsupported audio format")
	ErrESPIPE      = errors.New("illegal seek")
)
