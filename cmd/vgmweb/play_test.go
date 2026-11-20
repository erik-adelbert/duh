// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type testAudio struct {
	start  time.Time
	output bytes.Buffer
	chunks []int
	short  bool
	paused atomic.Bool
}

type controlledAudio struct {
	paused  atomic.Bool
	stopped atomic.Bool
	elapsed atomic.Int64
	writes  atomic.Int32
}

type finalEOFReader struct {
	data []byte
}

func (r *finalEOFReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (a *controlledAudio) Elapsed() time.Duration { return time.Duration(a.elapsed.Load()) }
func (a *controlledAudio) Paused() bool           { return a.paused.Load() }
func (a *controlledAudio) Stopped() bool          { return a.stopped.Load() }
func (a *controlledAudio) Write(p []byte) (int, error) {
	a.writes.Add(1)
	return len(p), nil
}

func (a *testAudio) Elapsed() time.Duration { return time.Since(a.start) }
func (a *testAudio) Paused() bool           { return a.paused.Load() }
func (a *testAudio) Stopped() bool          { return false }
func (a *testAudio) Write(p []byte) (int, error) {
	if a.short {
		return len(p) - 1, nil
	}
	a.chunks = append(a.chunks, len(p))
	return a.output.Write(p)
}

func TestPlayBatchesAndDrains(t *testing.T) {
	input := bytes.Repeat([]byte{1, 2, 3, 4}, 4096)
	audio := &testAudio{start: time.Now()}
	if err := play(bytes.NewReader(input), audio); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(audio.output.Bytes(), input) || len(audio.chunks) != 4 {
		t.Fatalf("incorrect output or batches: %v", audio.chunks)
	}
	for _, n := range audio.chunks {
		if n != audioChunkBytes {
			t.Fatalf("chunk size %d, want %d", n, audioChunkBytes)
		}
	}
	duration := time.Duration(int64(len(input)) * int64(time.Second) / audioBytesPerSecond)
	if audio.Elapsed() < duration {
		t.Fatal("playback returned before the audio drained")
	}
}

func TestPlayShortWrite(t *testing.T) {
	audio := &testAudio{start: time.Now(), short: true}
	if err := play(bytes.NewReader([]byte{1, 2, 3, 4}), audio); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("got %v, want short write", err)
	}
}

func TestPausePreventsReadsAndDefersDrain(t *testing.T) {
	audio := &controlledAudio{}
	audio.paused.Store(true)
	done := make(chan error, 1)
	go func() { done <- play(bytes.NewReader([]byte{1, 2, 3, 4}), audio) }()
	time.Sleep(60 * time.Millisecond)
	if audio.writes.Load() != 0 {
		t.Fatal("wrote audio while paused")
	}

	audio.paused.Store(false)
	deadline := time.After(time.Second)
	for audio.writes.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("playback did not resume")
		default:
			time.Sleep(time.Millisecond)
		}
		audio.paused.Store(true)
		select {
		case err := <-done:
			t.Fatalf("drained while paused: %v", err)
		case <-time.After(60 * time.Millisecond):
		}
		audio.elapsed.Store(int64(time.Second))
		audio.paused.Store(false)
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("playback did not finish after resuming")
		}
	}
}

func TestStopWhilePaused(t *testing.T) {
	audio := &controlledAudio{}
	audio.paused.Store(true)
	done := make(chan error, 1)
	go func() { done <- play(bytes.NewReader([]byte{1, 2, 3, 4}), audio) }()
	audio.stopped.Store(true)
	select {
	case err := <-done:
		if err != nil || audio.writes.Load() != 0 {
			t.Fatalf("stop while paused: error %v, writes %d", err, audio.writes.Load())
		}
	case <-time.After(time.Second):
		t.Fatal("playback did not stop while paused")
	}
}

func TestStopDuringDrain(t *testing.T) {
	audio := &controlledAudio{}
	done := make(chan error, 1)
	go func() { done <- play(&finalEOFReader{data: []byte{1, 2, 3, 4}}, audio) }()
	for deadline := time.Now().Add(time.Second); audio.writes.Load() == 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("audio was not written")
		}
	}
	audio.stopped.Store(true)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("playback did not stop during drain")
	}
}
