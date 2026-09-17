package mixer

import (
	"github.com/erik-adelbert/duh/internal/tracker/components/sample"
	ac "github.com/erik-adelbert/duh/internal/tracker/components/voice"
)

type Routine func([]Fp284, *ac.Cue, *ac.Gain, *ac.Filter, sample.Data, ac.Flags)
