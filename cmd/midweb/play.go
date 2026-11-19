// Original code Copyright (c) 2026 Erik Adelbert.
// Licensed under the BSD 3-Clause license; see ../../LICENCE.

package main

import (
	"io"
	"time"
)

const (
	// Keep in sync with the pool buffer size in docs/midweb/mid.html.
	audioChunkBytes     = 4 * 1024
	audioBytesPerSecond = 48000 * 4
	audioLead           = time.Duration(12 * 1024 * int64(time.Second) / audioBytesPerSecond)
)

type audioOutput interface {
	io.Writer
	Elapsed() time.Duration
}

func play(in io.Reader, audio audioOutput) error {
	buf := make([]byte, audioChunkBytes)
	var written int64
	start := audio.Elapsed()

	for {
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
			// Yield to the browser even while filling or catching up.
			time.Sleep(max(wait, time.Millisecond))
		}

		switch err {
		case nil:
			if n == 0 {
				return io.ErrNoProgress
			}
		case io.EOF:
			target := time.Duration(written * int64(time.Second) / audioBytesPerSecond)
			if wait := target - (audio.Elapsed() - start); wait > 0 {
				time.Sleep(wait)
			}
			return nil
		default:
			return err
		}
	}
}
