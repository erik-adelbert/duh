// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sample

type Flags uint32

const (
	Sample16Bit Flags = 1 << iota
	SampleStereo
	SampleLooped
	SampleBounceLoop
	SampleSustainLoop
)
