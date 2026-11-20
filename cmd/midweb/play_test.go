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
	err    error
	short  bool
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

func (a *testAudio) Elapsed() time.Duration { return time.Since(a.start) }
func (a *testAudio) Paused() bool           { return false }
func (a *testAudio) Stopped() bool          { return false }

func (a *testAudio) Write(p []byte) (int, error) {
	if a.err != nil {
		return 0, a.err
	}
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
	if !bytes.Equal(audio.output.Bytes(), input) {
		t.Fatal("audio output differs from input")
	}
	if len(audio.chunks) != 4 {
		t.Fatalf("got %d chunks, want 4", len(audio.chunks))
	}
	for _, size := range audio.chunks {
		if size != audioChunkBytes {
			t.Fatalf("chunk size %d, want %d", size, audioChunkBytes)
		}
	}
	duration := time.Duration(int64(len(input)) * int64(time.Second) / audioBytesPerSecond)
	if elapsed := audio.Elapsed(); elapsed < duration {
		t.Fatalf("playback returned after %s, before %s of audio drained", elapsed, duration)
	}
}

func TestPlayErrors(t *testing.T) {
	want := errors.New("write failed")
	for _, tt := range []struct {
		name  string
		audio *testAudio
		want  error
	}{
		{"write error", &testAudio{err: want}, want},
		{"short write", &testAudio{short: true}, io.ErrShortWrite},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.audio.start = time.Now()
			if err := play(bytes.NewReader([]byte{1, 2, 3, 4}), tt.audio); !errors.Is(err, tt.want) {
				t.Fatalf("error %v, want %v", err, tt.want)
			}
		})
	}
}

func TestPlayFinalReadWithEOF(t *testing.T) {
	input := bytes.Repeat([]byte{1, 2, 3, 4}, 1300)
	audio := &testAudio{start: time.Now()}
	if err := play(&finalEOFReader{data: input}, audio); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(audio.output.Bytes(), input) {
		t.Fatal("final read data was not fully written")
	}
	duration := time.Duration(int64(len(input)) * int64(time.Second) / audioBytesPerSecond)
	if elapsed := audio.Elapsed(); elapsed < duration {
		t.Fatalf("playback returned after %s, before %s of audio drained", elapsed, duration)
	}
}

func TestPlayEmptyInput(t *testing.T) {
	audio := &testAudio{start: time.Now()}
	if err := play(bytes.NewReader(nil), audio); err != nil {
		t.Fatal(err)
	}
	if audio.output.Len() != 0 {
		t.Fatal("empty input produced audio")
	}
}

type controlledAudio struct {
	paused       atomic.Bool
	stopped      atomic.Bool
	elapsed      atomic.Int64
	writes       atomic.Int32
	pauseOnWrite bool
}

func (a *controlledAudio) Elapsed() time.Duration { return time.Duration(a.elapsed.Load()) }
func (a *controlledAudio) Paused() bool           { return a.paused.Load() }
func (a *controlledAudio) Stopped() bool          { return a.stopped.Load() }
func (a *controlledAudio) Write(p []byte) (int, error) {
	if a.pauseOnWrite {
		a.paused.Store(true)
	}
	a.writes.Add(1)
	return len(p), nil
}

func TestPauseBeforeRead(t *testing.T) {
	audio := &controlledAudio{}
	audio.paused.Store(true)
	done := make(chan error, 1)
	go func() { done <- play(bytes.NewReader([]byte{1, 2, 3, 4}), audio) }()
	time.Sleep(60 * time.Millisecond)
	if audio.writes.Load() != 0 {
		t.Fatal("wrote audio while paused")
	}
	audio.elapsed.Store(int64(time.Second))
	audio.paused.Store(false)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("playback did not resume")
	}
}

func TestPauseDuringFinalDrain(t *testing.T) {
	audio := &controlledAudio{pauseOnWrite: true}
	done := make(chan error, 1)
	go func() { done <- play(&finalEOFReader{data: []byte{1, 2, 3, 4}}, audio) }()
	deadline := time.After(time.Second)
	for audio.writes.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("audio was not written")
		default:
			time.Sleep(time.Millisecond)
		}

	}
	select {
	case err := <-done:
		t.Fatalf("playback ended while paused: %v", err)
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
	audio := &controlledAudio{pauseOnWrite: true}
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
