// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webaudio

import (
	"fmt"
	"io"
	"time"
)

const ChunkBytes = 4 * 1024

type Output interface {
	io.Writer
	Elapsed() time.Duration
	Paused() bool
	Stopped() bool
}

func Play(in io.Reader, audio Output, bytesPerSecond int) error {
	if bytesPerSecond <= 0 {
		return fmt.Errorf("invalid audio byte rate: %d", bytesPerSecond)
	}
	buf := make([]byte, ChunkBytes)
	lead := time.Duration(3 * ChunkBytes * int64(time.Second) / int64(bytesPerSecond))
	var written int64
	start := audio.Elapsed()

	for {
		if audio.Stopped() {
			return nil
		}
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
			target := time.Duration(written * int64(time.Second) / int64(bytesPerSecond))
			wait := target - (audio.Elapsed() - start) - lead
			time.Sleep(max(wait, time.Millisecond))
		}
		switch err {
		case nil:
			if n == 0 {
				return io.ErrNoProgress
			}
		case io.EOF:
			target := time.Duration(written * int64(time.Second) / int64(bytesPerSecond))
			for {
				if audio.Stopped() {
					return nil
				}
				if audio.Paused() {
					time.Sleep(20 * time.Millisecond)
					continue
				}
				wait := target - (audio.Elapsed() - start)
				if wait <= 0 {
					return nil
				}
				time.Sleep(min(wait, 20*time.Millisecond))
			}
		default:
			return err
		}
	}
}
