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
