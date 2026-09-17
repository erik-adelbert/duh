package dsp

import "github.com/erik-adelbert/duh/pkg/xerror"

var errp = xerror.NewPolicy()

var ErrDSP = errp.Global("DSP error")
