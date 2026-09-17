// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analyzer

import "github.com/erik-adelbert/duh/pkg/xerror"

var errp = xerror.NewPolicy()

var ErrAnalyzer = errp.Global("analyzer error")

var (
	ErrNew  = errp.Export("new error")
	ErrRead = errp.Export("read error")
)

var mkError = errp.MkError
