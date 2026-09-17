// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package audiocontext

type MixingSetup struct {
	SampleRate
	BitDepth
	ChannelCount   uint32
	MaxMixChannels uint32
}

type Delayed struct {
	Depth uint32
	Delay uint32
}

type Ranged struct {
	Depth uint32
	Range uint32
}

type AudioEffects struct {
	Prologic         Delayed
	Reverb           Delayed
	XBass            Ranged
	StereoSeparation uint32
}

type ProcessingSettings struct {
	SoundSetup        uint32
	VolumeRampSamples uint32
	StreamVolume      int32
}

type SampleRate uint32

const (
	SampleRate22K  SampleRate = 22_050
	SampleRate441K SampleRate = 44_100
	SampleRate48K  SampleRate = 48_000
)

type BitDepth uint32

const (
	Bits8  BitDepth = 8
	Bits16 BitDepth = 16
)
