// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pattern

import "github.com/erik-adelbert/duh/internal/help"

type Cell struct {
	Note   uint8
	Instru uint8
	Volume uint8
	Effect uint8
	Param  uint8
}

type Data = help.Matrix[Cell]

type Metadata struct {
	Name, Comment string
}

type State struct {
	CurrentRow int
	IsPlaying  bool
}
