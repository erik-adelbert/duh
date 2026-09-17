// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mp3

import (
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

var ErrMP3 = errp.Global("MP3 error")

func init() {
	errp.Bubble(io.EOF)
}

var (
	ErrEncode = errp.Export("encode error")
	ErrDecode = errp.Export("decode error")
	ErrSeek   = errp.Export("seek error")
)

var mkError = errp.MkError
