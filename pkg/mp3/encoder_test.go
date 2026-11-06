// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mp3

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/erik-adelbert/duh/pkg/pcm"
)

func TestEncoderWriteIsIndependentOfInputChunking(t *testing.T) {
	format, err := pcm.NewFormat('s', 16, 2, 44100)
	if err != nil {
		t.Fatal(err)
	}

	input := make([]byte, 5003*int(format.FrameSize()))
	for i := 0; i < len(input)/2; i++ {
		value := int16((i*997)%65536 - 32768)
		binary.LittleEndian.PutUint16(input[i*2:], uint16(value))
	}

	encode := func(chunks []int) []byte {
		t.Helper()
		var output bytes.Buffer
		encoder, err := Encode(&output, format)
		if err != nil {
			t.Fatal(err)
		}

		offset := 0
		for _, size := range chunks {
			end := min(offset+size, len(input))
			n, err := encoder.Write(input[offset:end])
			if err != nil {
				t.Fatal(err)
			}
			if n != end-offset {
				t.Fatalf("Write consumed %d bytes, want %d", n, end-offset)
			}
			offset = end
		}
		if offset < len(input) {
			n, err := encoder.Write(input[offset:])
			if err != nil {
				t.Fatal(err)
			}
			if n != len(input)-offset {
				t.Fatalf("final Write consumed %d bytes, want %d", n, len(input)-offset)
			}
		}
		if err := encoder.Close(); err != nil {
			t.Fatal(err)
		}
		return output.Bytes()
	}

	whole := encode([]int{len(input)})
	fragmented := encode([]int{1, 997, 3, 4093, 7, 2048})
	if len(whole) == 0 {
		t.Fatal("encoder produced no MP3 output")
	}
	if !bytes.Equal(whole, fragmented) {
		t.Fatalf("output differs by write chunking: whole=%d bytes fragmented=%d bytes", len(whole), len(fragmented))
	}
}
