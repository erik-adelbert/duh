// Copyright (c) 2026 Erik Adelbert. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pcm

import (
	"bytes"
	"errors"
	"io"
	"math"
	"testing"
)

type positionSeeker struct {
	calls int
	pos   int64
}

func (s *positionSeeker) Read([]byte) (int, error) { return 0, io.EOF }
func (s *positionSeeker) Seek(offset int64, whence int) (int64, error) {
	s.calls++
	if whence == io.SeekStart {
		s.pos = offset
	}
	return s.pos, nil
}

func TestConvReaderSeekOffsetOverflow(t *testing.T) {
	in := mustReaderFormat(t, SLE16Stereo44k)
	out := mustReaderFormat(t, U8Mono8k)
	out.sampleRate = in.sampleRate
	conv, err := NewConverter(out, in)
	if err != nil {
		t.Fatal(err)
	}
	source := &positionSeeker{}
	reader := OpenConv(source, conv)
	if _, err := reader.Seek(math.MaxInt64, io.SeekStart); !errors.Is(err, ErrOffsetOverflow) || source.calls != 0 {
		t.Fatalf("seek overflow = %v, source calls %d", err, source.calls)
	}
	if _, err := reader.Seek(math.MinInt64, io.SeekCurrent); !errors.Is(err, ErrOffsetOverflow) || source.calls != 0 {
		t.Fatalf("negative seek overflow = %v, source calls %d", err, source.calls)
	}

	in, out = out, in
	conv, err = NewConverter(out, in)
	if err != nil {
		t.Fatal(err)
	}
	source = &positionSeeker{pos: math.MaxInt64}
	reader = OpenConv(source, conv)
	reader.pending = []byte{1, 2}
	if _, err := reader.Seek(0, io.SeekCurrent); !errors.Is(err, ErrOffsetOverflow) || source.calls != 1 {
		t.Fatalf("result overflow = %v, source calls %d", err, source.calls)
	}
	if !bytes.Equal(reader.pending, nil) {
		t.Fatal("conversion state not reset after source moved")
	}
}
