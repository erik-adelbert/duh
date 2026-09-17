// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package a440 generates reference signals in PCM format.
// The following signals are supported:
//   - sine wave
//   - square wave
//   - triangle wave
//   - sawtooth wave
//   - sweep (linear and logarithmic)
//   - white noise
//   - pink noise
//   - silence
//   - impulse
package a440

import (
	"io"
	"os"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

// Example demonstrates how to generate a 440 Hz sine wave and write it to a PCM file.
func Example(path string) error {
	f, err := pcm.ParseFormat("s16c2r44100")

	if err != nil {
		return err
	}

	// Create a new 440 Hz sine wave PCM stream generator
	sineWave := NewSine(f, 5*time.Second, 440)

	// You can also generate your own signal by providing
	// a float iterator check out NewReader()

	out, _ := os.Create(path)
	defer out.Close() //nolint:errcheck // example

	_, err = io.Copy(out, sineWave)
	return err
}
