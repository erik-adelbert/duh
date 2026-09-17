package aac

import (
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

// ErrAAC is returned as a global aac error.
var ErrAAC = errp.Global("aac error")

func init() {
	errp.Bubble(io.EOF)
}

var (
	ErrFormat = errp.Export("invalid format")
	ErrDecode = errp.Export("decode error")
	ErrEncode = errp.Export("write error")
)

var (
	mkError  = errp.MkError
	mkErrorf = errp.MkErrorf
)
