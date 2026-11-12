// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package backend

import (
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"testing"
	"time"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func TestOtoPrepareReader(t *testing.T) {
	inputFormat, err := pcm.NewFormat(pcm.SignedIntBE, 16, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}

	outputFormat, err := pcm.NewFormat(pcm.Float, 32, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("converts input to output", func(t *testing.T) {
		backend := OtoBackend{Format: inputFormat, ofmt: outputFormat}
		source := bytes.NewReader([]byte{0x40, 0x00, 0x40, 0x00})
		reader, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if seeker == nil || reader != seeker {
			t.Fatal("seekable input should produce a seekable converted reader")
		}

		var output [4]byte
		if _, err := io.ReadFull(reader, output[:]); err != nil {
			t.Fatal(err)
		}
		if got := math.Float32frombits(binary.LittleEndian.Uint32(output[:])); got != 0.5 {
			t.Fatalf("converted sample = %v, want 0.5", got)
		}
	})

	t.Run("unchanged format", func(t *testing.T) {
		backend := OtoBackend{Format: outputFormat, ofmt: outputFormat}
		source := bytes.NewReader([]byte{1, 2, 3, 4})
		reader, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if reader != source || seeker != source {
			t.Fatal("matching formats should preserve the original reader")
		}
	})

	t.Run("non-seekable input", func(t *testing.T) {
		backend := OtoBackend{Format: inputFormat, ofmt: outputFormat}
		source := struct{ io.Reader }{bytes.NewReader([]byte{0x40, 0x00})}
		_, seeker, err := backend.prepReader(source)
		if err != nil {
			t.Fatal(err)
		}
		if seeker != nil {
			t.Fatal("non-seekable input should not produce a seeker")
		}
	})
}

func TestOtoPlayerDurationUsesOutputFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{"converted depth", "S16c1r44100", "f32c1r44100"},
		{"converted channels", "s16c4r44100", "f32c2r44100"},
		{"unchanged", "s16c2r44100", "s16c2r44100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ifmt, err := pcm.ParseFormat(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			ofmt, err := pcm.ParseFormat(tt.out)
			if err != nil {
				t.Fatal(err)
			}
			backend := OtoBackend{Format: ifmt, ofmt: ofmt}
			source := bytes.NewReader(make([]byte, int(ifmt.BufferSize(time.Second))))
			_, seeker, err := backend.prepReader(source)
			if err != nil {
				t.Fatal(err)
			}
			player := OtoPlayer{Format: ifmt, ofmt: ofmt, rs: seeker}
			if got := player.Duration(); got != time.Second {
				t.Fatalf("duration = %v, want %v", got, time.Second)
			}
			if got := player.Format; got != ifmt {
				t.Fatalf("player format = %v, want input format %v", got, ifmt)
			}
		})
	}
}
