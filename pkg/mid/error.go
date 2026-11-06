package mid

import (
	"errors"
	"io"

	"github.com/erik-adelbert/duh/pkg/xerror"
)

var errp = xerror.NewPolicy()

func init() {
	errp.Bubble(io.EOF)
	errp.Bubble(io.ErrUnexpectedEOF)
}

var (
	ErrMidi    = errp.Global("midi error")
	ErrNotRIFF = errors.New("not a RIFF file")
)

var mkError = errp.MkError
