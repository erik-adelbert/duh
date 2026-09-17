// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package voice

import (
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
)

// MixingControl holds core state and routing.
type MixingControl struct {
	Active bool
	Master uint32 // master channel
	Flags  uint32
	Plugin id.ID
}

type MixingParams struct {
	Length int32 // length in samples
	Filter
	Offset
	Gain
	Loop
	Cue
}

type Gain struct {
	Ramp
	Volume core.Stereo[Fp284]
}

type Ramp struct {
	core.Stereo[Fp284]
	Length int32
	Volume core.Stereo[Fp284]
}

type Loop = core.Span[int32]

type Cue struct {
	IPart int32
	FPart int32 // actually 16-bit
	Step  int32 // fixed 16.16
}

type Offset core.Stereo[Fp284]

type Filter struct {
	Y  [4]Fp284 // Y1, Y2, Y3, Y4
	A0 Fp284
	B0 Fp284
	B1 Fp284
}

type Balance struct {
	core.Balance[Fp284]
	New struct {
		Balance core.Balance[Fp284]
		Volume  core.Stereo[Fp284]
	}
	SubVols struct {
		FadeOut Fp284
		Global  Fp284
		Instru  Fp284
	}
	FineTune  Fp284
	Transpose Fp284
}

type MixerRoutine PatchID
