// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package audio

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()

	if c.Channels != 2 {
		t.Errorf("expected Channels=2, got %d", c.Channels)
	}

	if c.BitDepth != 16 {
		t.Errorf("expected BitDepth=16, got %d", c.BitDepth)
	}

	if c.SampleRate != 44_100 {
		t.Errorf("expected SampleRate=44100, got %d", c.SampleRate)
	}

	if c.ResamplingMode != ResampleLinear {
		t.Errorf("expected ResamplingMode=ResampleLinear, got %v", c.ResamplingMode)
	}

	if c.StereoSeparation != 0.5 {
		t.Errorf("expected StereoSeparation=0.5, got %v", c.StereoSeparation)
	}

	if c.MaxMixingChannels != 32 {
		t.Errorf("expected MaxMixingChannels=32, got %v", c.MaxMixingChannels)
	}

	if c.Reverb.Depth != 0.0 {
		t.Errorf("expected Reverb.Depth=0.0, got %v", c.Reverb.Depth)
	}

	if c.XBass.Depth != 0.0 {
		t.Errorf("expected XBass.Depth=0.0, got %v", c.XBass.Depth)
	}

	if c.Surround.Depth != 0.0 {
		t.Errorf("expected Surround.Depth=0.0, got %v", c.Surround.Depth)
	}
}
