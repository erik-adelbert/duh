// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package player

import "errors"

var (
	ErrNoOptions      = errors.New("player: no audio options")
	ErrInvalidSeek    = errors.New("player: invalid seek operation")
	ErrUnsupportedFmt = errors.New("player: unsupported audio format")
)
