// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcm

import (
	"bytes"
	"testing"
	"time"
)

func TestTapStartSendsActualByteLength(t *testing.T) {
	format, err := ParseFormat(SLE16Stereo44k)
	if err != nil {
		t.Fatalf("FormatFromString: %v", err)
	}

	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 2)
	got := make(chan []byte, 1)

	tap, err := NewTap(format, 1*time.Millisecond, 100, time.Millisecond)
	if err != nil {
		t.Fatalf("NewTap: %v", err)
	}

	tap.updateFn = func(p []byte) {
		got <- append([]byte(nil), p...)
	}

	_, _ = tap.Write(payload)
	tap.Start()

	tap.Start() // Trigger the update to send the data to the callback
	defer tap.Stop()

	select {
	case p := <-got:
		if gotLen, wantLen := len(p), len(payload); gotLen != wantLen {
			t.Fatalf("callback payload length = %d, want %d", gotLen, wantLen)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for tap callback")
	}
}
