// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package module

import (
	"sync"

	"github.com/erik-adelbert/duh/internal/audio"
	"github.com/erik-adelbert/duh/internal/id"
	"github.com/erik-adelbert/duh/internal/tracker/components/core"
)

type MixBus struct {
	SoundBuf, ReverbBuf, RearBuf *Buffer
	Voices                       []id.ID
	DCOff                        core.Stereo[Fp284]
	RVBOff                       core.Stereo[Fp284]
	VUPeak                       core.Stereo[Fp284]
	AGC                          AGCState
	BufferPos                    int
	ChunkSize                    int
	ReverbSend                   int
	once                         *sync.Once
	closer                       func()
}

const AudioBufferSize = 1024

func NewMixBus() *MixBus {
	return &MixBus{
		// name:   name,
		once:   &sync.Once{},
		closer: func() {},
	}
}

func (m *MixBus) Initialize() {
	m.once.Do(func() {
		soundBuf, soundBufCloser := audio.NewBuffer(2 * AudioBufferSize)
		reverBuf, reverBufCloser := audio.NewBuffer(AudioBufferSize)
		rearBuf, rearBufCloser := audio.NewBuffer(AudioBufferSize)
		m.SoundBuf = soundBuf
		m.ReverbBuf = reverBuf
		m.RearBuf = rearBuf
		m.closer = func() {
			soundBufCloser()
			reverBufCloser()
			rearBufCloser()
		}
	})
}

func (m *MixBus) Close() {
	m.closer()
}

type (
	Fp284  audio.Fp284
	Buffer = audio.Buffer
)
