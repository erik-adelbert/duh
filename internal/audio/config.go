// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/erik-adelbert/duh/internal/help"
)

type Config struct {
	Features
	Channels          int
	BitDepth          int
	SampleRate        int
	ResamplingMode    int
	StereoSeparation  float64
	MaxMixingChannels int
	Reverb            Delayed
	XBass             Delayed
	Surround          Delayed
	Looping           int
}

func DefaultConfig() Config {
	return Config{
		Features:          EnableOverSampling | EnableNoiseReduction,
		Channels:          2,
		BitDepth:          16,
		SampleRate:        44_100,
		ResamplingMode:    ResampleLinear,
		StereoSeparation:  0.5,
		MaxMixingChannels: 32,
	}
}

const (
	ResampleNearest int = iota
	ResampleLinear
	ResampleSpline
	ResampleFIR
)

type Delayed struct {
	Depth float64
	Delay time.Duration
}

func (d Delayed) SafeDelay(min, max time.Duration) Delayed {
	d.Depth = help.Clamp(d.Depth, 0, 1)
	d.Delay = help.Clamp(d.Delay, min, max)

	return d
}

type Features int

const (
	EnableOverSampling Features = 1 << iota
	EnableNoiseReduction
	EnableReverb
	EnableXBass
	EnableSurround
)

var featureNames = []struct {
	feature Features
	name    string
}{
	{EnableOverSampling, "oversampling"},
	{EnableNoiseReduction, "noise_reduction"},
	{EnableReverb, "reverb"},
	{EnableXBass, "xbass"},
	{EnableSurround, "surround"},
}

func (f Features) MarshalJSON() ([]byte, error) {
	var names []string

	for _, fn := range featureNames {
		if f&fn.feature != 0 {
			names = append(names, fn.name)
		}
	}

	return json.Marshal(strings.Join(names, "|"))
}

func (f *Features) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parts := strings.Split(s, "|")

	var features Features

	for _, part := range parts {
		for _, fn := range featureNames {
			if part == fn.name {
				features |= fn.feature
			}
		}
	}

	*f = features

	return nil
}
