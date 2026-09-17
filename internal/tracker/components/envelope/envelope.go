// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package envelope

import "github.com/erik-adelbert/duh/internal/tracker/components/core"

type Envelope struct {
	Points
	Loops
	Flags
	Size
}

type Loops core.Voice[core.LoopSustain[uint8]]
type Points core.Voice[[]uint16]
type Env core.Voice[[]uint8]
type Size core.Voice[uint8]
