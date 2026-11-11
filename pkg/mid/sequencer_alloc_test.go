// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mid

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
	"time"
)

func testSMFTracks(tracks ...[]byte) []byte {
	data := []byte{'M', 'T', 'h', 'd', 0, 0, 0, 6, 0, 1, 0, 0, 0, 96}
	binary.BigEndian.PutUint16(data[10:12], uint16(len(tracks)))
	for _, track := range tracks {
		data = append(data, 'M', 'T', 'r', 'k', 0, 0, 0, 0)
		binary.BigEndian.PutUint32(data[len(data)-4:], uint32(len(track)))
		data = append(data, track...)
	}
	return data
}

func TestPositionSnapshotReadersAreIndependent(t *testing.T) {
	source := position{tracks: []trackInfo{{Reader: bytes.NewReader([]byte{1, 2})}, {}}}
	var snapshot position
	snapshot.copyFrom(&source)
	_, _ = source.tracks[0].ReadByte()
	if got, _ := snapshot.tracks[0].ReadByte(); got != 1 {
		t.Fatalf("snapshot read = %d, want 1", got)
	}
	if snapshot.tracks[1].Reader != nil {
		t.Fatal("snapshot changed nil reader")
	}

	source.tracks = append(source.tracks, trackInfo{Reader: bytes.NewReader([]byte{3})})
	snapshot.copyFrom(&source)
	if got, _ := snapshot.tracks[0].ReadByte(); got != 2 {
		t.Fatalf("resized snapshot read = %d, want 2", got)
	}
	if got, _ := snapshot.tracks[2].ReadByte(); got != 3 {
		t.Fatalf("new track read = %d, want 3", got)
	}
}

func TestReadDelayPeekRestoresReader(t *testing.T) {
	for _, tt := range []struct {
		name string
		data []byte
		want int64
	}{
		{"end of track", []byte{96, 0xff, 0x2f, 0}, 0},
		{"other event", []byte{96, 0x90, 60}, 96},
		{"partial event", []byte{96, 0xff}, 96},
		{"no event", []byte{96}, 96},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := newSequencer(&testSynth{})
			reader := bytes.NewReader(tt.data)
			s.pos.cur.tracks = []trackInfo{{Reader: reader}}
			if got := s.ReadDelay(0); got != tt.want {
				t.Errorf("delay = %d, want %d", got, tt.want)
			}
			if got := reader.Size() - int64(reader.Len()); got != 1 {
				t.Errorf("reader position = %d, want 1", got)
			}
		})
	}
}

func TestSequencerMultitrackLoopSnapshot(t *testing.T) {
	first := []byte{
		0, 0xff, 0x51, 3, 0x07, 0xa1, 0x20,
		0, 0x90, 50, 100,
		96, 0xff, 0x06, 9, 'l', 'o', 'o', 'p', 'S', 't', 'a', 'r', 't',
		0, 0x90, 60, 100,
		96, 0xff, 0x06, 7, 'l', 'o', 'o', 'p', 'E', 'n', 'd',
		0, 0xff, 0x2f, 0,
	}
	second := []byte{
		0, 0x90, 70, 100,
		96, 0x90, 72, 100,
		96, 0x80, 72, 0,
		96, 0xff, 0x2f, 0,
	}
	target := &testSynth{}
	s := newSequencer(target)
	if err := s.openReader(bytes.NewReader(testSMFTracks(first, second))); err != nil {
		t.Fatal(err)
	}
	s.Tick(0)
	for range 4 {
		s.Tick(500 * time.Millisecond)
	}
	want := []string{
		"on:0:50:100", "on:0:70:100",
		"on:0:60:100", "on:0:72:100",
		"off:0:72", "on:0:60:100", "on:0:72:100",
		"off:0:72", "on:0:60:100", "on:0:72:100",
		"off:0:72", "on:0:60:100", "on:0:72:100",
	}
	if !reflect.DeepEqual(target.events, want) {
		t.Fatalf("loop events = %v, want %v", target.events, want)
	}
}

func allocationTestSequencer(tb testing.TB) *Sequencer {
	tb.Helper()
	track := []byte{
		0, 0xff, 0x06, 9, 'l', 'o', 'o', 'p', 'S', 't', 'a', 'r', 't',
		96, 0xff, 0x06, 7, 'l', 'o', 'o', 'p', 'E', 'n', 'd',
		0, 0xff, 0x2f, 0,
	}
	s := newSequencer(&testSynth{})
	if err := s.openReader(bytes.NewReader(testSMFTracks(track, track))); err != nil {
		tb.Fatal(err)
	}
	s.pos.cur.isPlaying = true
	s.ProcessEvents()
	s.ProcessEvents()
	return s
}

func TestSequencerLoopSnapshotsDoNotAllocateAfterWarmup(t *testing.T) {
	s := allocationTestSequencer(t)
	if got := testing.AllocsPerRun(100, func() {
		s.ProcessEvents()
		s.ProcessEvents()
	}); got != 0 {
		t.Fatalf("loop iteration allocations = %g, want 0", got)
	}
}

func BenchmarkSequencerLoopSnapshots(b *testing.B) {
	s := allocationTestSequencer(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		s.ProcessEvents()
		s.ProcessEvents()
	}
}
