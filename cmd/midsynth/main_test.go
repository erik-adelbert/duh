package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/erik-adelbert/duh/pkg/mid"
)

func TestGetArgs(t *testing.T) {
	for _, tt := range []struct {
		name string
		opts []string
		want args
	}{
		{"defaults", nil, args{portNumber: -1, cards: 4, fourOps: -1}},
		{"options", []string{"-list", "-in", "2", "-bank", "79", "-cards", "4", "-four-ops", "3"},
			args{list: true, portNumber: 2, bank: 79, cards: 4, fourOps: 3}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := os.Args
			os.Args = append([]string{"midsynth"}, tt.opts...)
			t.Cleanup(func() { os.Args = original })
			if got := getArgs(); got != tt.want {
				t.Fatalf("getArgs() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHandleMIDI(t *testing.T) {
	tests := []struct {
		name string
		msg  []byte
		want func(*mid.Synth) error
	}{
		{"note on", []byte{0x91, 60, 100}, func(s *mid.Synth) error { _, err := s.RTNoteOn(1, 60, 100); return err }},
		{"note off", []byte{0x81, 60, 45}, func(s *mid.Synth) error { return s.RTNoteOff(1, 60) }},
		{"poly pressure", []byte{0xa1, 60, 45}, func(s *mid.Synth) error { return s.RTNoteAfterTouch(1, 60, 45) }},
		{"controller", []byte{0xb1, 7, 45}, func(s *mid.Synth) error { return s.RTControllerChange(1, 7, 45) }},
		{"program", []byte{0xc1, 7}, func(s *mid.Synth) error { return s.RTPatchChange(1, 7) }},
		{"channel pressure", []byte{0xd1, 45}, func(s *mid.Synth) error { return s.RTChannelAfterTouch(1, 45) }},
		{"pitch bend", []byte{0xe1, 1, 70}, func(s *mid.Synth) error { return s.RTPitchBendML(1, 70, 1) }},
		{"sysex", []byte{0xf0, 0x7d, 'D', 'U', 'H', 1, 1, 0, 79, 0xf7}, func(s *mid.Synth) error {
			_, err := s.RTSystemExclusive([]byte{0xf0, 0x7d, 'D', 'U', 'H', 1, 1, 0, 79, 0xf7})
			return err
		}},
		{"clock", []byte{0xf8}, func(*mid.Synth) error { return nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, want := mid.NewSynth(1, 0, 0), mid.NewSynth(1, 0, 0)
			for _, s := range []*mid.Synth{got, want} {
				if _, err := s.RTNoteOn(1, 60, 100); err != nil {
					t.Fatal(err)
				}
			}
			if err := handleMIDI(got, tt.msg); err != nil {
				t.Fatal(err)
			}
			if err := tt.want(want); err != nil {
				t.Fatal(err)
			}
			a, b := make([]int16, 2048), make([]int16, 2048)
			if _, err := got.RTGenerate(a); err != nil {
				t.Fatal(err)
			}
			if _, err := want.RTGenerate(b); err != nil {
				t.Fatal(err)
			}
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("sample %d: got %d, want %d", i, a[i], b[i])
				}
			}
		})
	}
}

func TestHandleMIDIInvalid(t *testing.T) {
	for _, msg := range [][]byte{nil, {0x90, 60}, {0x80, 60, 128}, {0xc0, 128}, {0xf0, 1, 2}} {
		if err := handleMIDI(mid.NewSynth(1, 0, 0), msg); err == nil {
			t.Errorf("accepted invalid MIDI message % X", msg)
		}
	}
}

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(p []byte) (int, error) {
	return w.Buffer.Write(p[:min(3, len(p))])
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriteAll(t *testing.T) {
	var w shortWriter
	if err := writeAll(&w, []byte{1, 2, 3, 4, 5}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.Bytes(), []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("output %v", w.Bytes())
	}
	if err := writeAll(zeroWriter{}, []byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("zero write: %v", err)
	}
}

type cancellingWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *cancellingWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.cancel()
	return n, err
}

func TestRenderPCMAndCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := mid.NewSynth(1, 0, 0)
	if _, err := s.RTNoteOn(0, 60, 100); err != nil {
		t.Fatal(err)
	}
	var w cancellingWriter
	w.cancel = cancel
	if err := render(ctx, s, &w, make(chan []byte), make(chan error)); err != nil {
		t.Fatal(err)
	}
	if w.Len() != blockFrames*4 {
		t.Fatalf("bytes = %d, want %d", w.Len(), blockFrames*4)
	}
	if bytes.Equal(w.Bytes(), make([]byte, w.Len())) {
		t.Fatal("note rendered as silence")
	}
}

func TestRenderReportsInputFailure(t *testing.T) {
	failures := make(chan error, 1)
	want := errors.New("input failed")
	failures <- want
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := render(ctx, mid.NewSynth(1, 0, 0), io.Discard, make(chan []byte), failures); !errors.Is(err, want) {
		t.Fatalf("render error = %v", err)
	}
}
