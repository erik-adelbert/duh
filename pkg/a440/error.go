// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a440

import (
	"errors"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

var ErrFreq = errp.Global("freq error")

var (
	ErrParse = errp.Export("parsing error")
	ErrSet   = errp.Export("set error")
)

var errSPN = errors.New("invalid SPN")

var mkError = errp.MkError
