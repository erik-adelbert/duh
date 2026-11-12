// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"errors"

	o "github.com/erik-adelbert/duh/pkg/offset"
	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

var ErrBackend = errp.Global("backend error")

func init() {
	errp.Bubble(ErrESPIPE)
}

var (
	ErrSeek   = errp.Export("seek error")
	ErrESPIPE = errors.New("illegal seek")
	// ErrOffsetOverflow indicates that a position cannot fit in an int64.
	ErrOffsetOverflow = o.ErrOverflow
)

var mkError = errp.MkError
