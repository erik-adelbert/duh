// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd_test

import (
	"bytes"
	"encoding/binary"
	"os/exec"
	"testing"
)

func TestEncodersConvertInputFormat(t *testing.T) {
	const frames = 4096
	input8 := make([]byte, frames)
	input16 := make([]byte, frames*2)
	for i := range input8 {
		input8[i] = byte(96 + i%64)
		binary.LittleEndian.PutUint16(input16[i*2:], uint16(int16(int(input8[i])-128)<<8))
	}

	for _, tc := range []struct {
		name   string
		output string
	}{
		{"wavenc", "s16c1r44100"},
		{"mp3enc", "s16c1r44100"},
		{"aacenc", "s16c1r44100b64000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(input []byte, format string) []byte {
				t.Helper()
				cmd := exec.Command("go", "run", "./"+tc.name, "-if", format, "-of", tc.output)
				cmd.Stdin = bytes.NewReader(input)
				var stderr bytes.Buffer
				cmd.Stderr = &stderr
				output, err := cmd.Output()
				if err != nil {
					t.Fatalf("%s: %v: %s", tc.name, err, stderr.String())
				}
				if len(output) == 0 {
					t.Fatal("encoder produced no output")
				}
				return output
			}

			want := run(input16, "s16c1r44100")
			got := run(input8, "u8c1r44100")
			if !bytes.Equal(got, want) {
				t.Fatalf("encoding converted PCM differs from equivalent s16 input: got %d bytes, want %d", len(got), len(want))
			}
			if tc.name == "wavenc" && (len(got) < 44 || !bytes.Equal(got[44:], input16)) {
				t.Fatal("WAV PCM payload differs from the expected samples")
			}
		})
	}
}
