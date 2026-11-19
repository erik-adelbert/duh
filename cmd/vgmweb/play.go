// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"time"
)

const (
	audioChunkBytes     = 4 * 1024
	audioBytesPerSecond = 44100 * 4
	audioLead           = time.Duration(12 * 1024 * int64(time.Second) / audioBytesPerSecond)
)

type audioOutput interface {
	io.Writer
	Elapsed() time.Duration
	Paused() bool
}

func play(in io.Reader, audio audioOutput) error {
	buf := make([]byte, audioChunkBytes)
	var written int64
	start := audio.Elapsed()
	for {
		if audio.Paused() {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		n, err := in.Read(buf)
		if n > 0 {
			nwritten, writeErr := audio.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			if nwritten != n {
				return io.ErrShortWrite
			}
			written += int64(n)
			target := time.Duration(written * int64(time.Second) / audioBytesPerSecond)
			wait := target - (audio.Elapsed() - start) - audioLead
			time.Sleep(max(wait, time.Millisecond))
		}
		switch err {
		case nil:
			if n == 0 {
				return io.ErrNoProgress
			}
		case io.EOF:
			target := time.Duration(written * int64(time.Second) / audioBytesPerSecond)
			for {
				if audio.Paused() {
					time.Sleep(20 * time.Millisecond)
					continue
				}
				wait := target - (audio.Elapsed() - start)
				if wait <= 0 {
					break
				}
				time.Sleep(min(wait, 20*time.Millisecond))
			}
			return nil
		default:
			return err
		}
	}
}
