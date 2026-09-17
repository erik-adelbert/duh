// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package module

type Counts struct {
	Voices  uint32
	Samples uint32
	Instrus uint32
}

type Metadata struct {
	Status
	Format
	Counts
	Name  string
	Speed uint32
	Tempo uint32
}

type Playback struct {
	Order int
	Row   int
	Tick  int
	Speed int
	Tempo int
}

type AGCState struct {
	Gain, Envelope float64
}

type Status int

const (
	StatusPaused Status = iota
	StatusMixing
	StatusDSPProcessing
	StatusFXProcessing
	StatusPostProcessing
)
